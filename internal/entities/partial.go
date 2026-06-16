package entities

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

// httpMethods — ключи операции внутри path item в OpenAPI; остальные ключи
// (parameters, $ref, summary, servers) относятся ко всему пути, не к методу.
var httpMethods = map[string]bool{
	"get": true, "put": true, "post": true, "delete": true,
	"options": true, "head": true, "patch": true, "trace": true,
}

// SelectMethods возвращает частичный контракт — копию, в которой остались только
// операции с перечисленными operationId. Каждая оставленная операция сохраняется
// целиком, а определения (`components`), на которые из неё больше никто не
// ссылается, отбрасываются — срез самодостаточен и при этом меньше исходного.
//
// Если запрошен метод, которого в контракте нет, возвращается *UnknownMethodsError
// со списком ненайденных — частичный/пустой результат наружу не отдаётся.
func (p *Protocol) SelectMethods(methods []string) (*Protocol, error) {
	want := make(map[string]bool, len(methods))
	for _, m := range methods {
		if m = strings.TrimSpace(m); m != "" {
			want[m] = true
		}
	}
	if len(want) == 0 {
		return nil, ErrNoMethodsSelected
	}

	var doc map[string]any
	if err := json.Unmarshal(p.Document, &doc); err != nil {
		return nil, ErrInvalidProtocol
	}
	paths, ok := doc["paths"].(map[string]any)
	if !ok {
		return nil, ErrInvalidProtocol
	}

	found := make(map[string]bool, len(want))
	newPaths := make(map[string]any, len(paths))
	for path, itemAny := range paths {
		item, ok := itemAny.(map[string]any)
		if !ok {
			continue
		}
		keptOps := make(map[string]any)
		for key, val := range item {
			if !httpMethods[strings.ToLower(key)] {
				continue
			}
			op, ok := val.(map[string]any)
			if !ok {
				continue
			}
			if id, _ := op["operationId"].(string); id != "" && want[id] {
				keptOps[key] = val
				found[id] = true
			}
		}
		if len(keptOps) == 0 {
			continue
		}
		for key, val := range item { // вернуть общие для пути ключи к оставленным операциям
			if !httpMethods[strings.ToLower(key)] {
				keptOps[key] = val
			}
		}
		newPaths[path] = keptOps
	}

	var missing []string
	for id := range want {
		if !found[id] {
			missing = append(missing, id)
		}
	}
	if len(missing) > 0 {
		sort.Strings(missing)
		return nil, &UnknownMethodsError{Methods: missing}
	}

	doc["paths"] = newPaths
	pruneComponents(doc, newPaths)

	document, err := json.Marshal(doc)
	if err != nil {
		return nil, fmt.Errorf("сериализация частичного контракта: %w", err)
	}

	selected := *p
	selected.Document = document
	return &selected, nil
}

// pruneComponents выкидывает из doc["components"] всё, на что из оставшихся путей
// не ведёт ни одна транзитивная ссылка $ref. securitySchemes ссылаются по имени из
// блоков security, поэтому отбираются отдельно по имени.
func pruneComponents(doc map[string]any, paths map[string]any) {
	components, ok := doc["components"].(map[string]any)
	if !ok {
		return
	}

	reachable := map[string]bool{}
	queue := collectRefs(paths)
	for len(queue) > 0 {
		ref := queue[len(queue)-1]
		queue = queue[:len(queue)-1]
		section, name, ok := parseComponentRef(ref)
		if !ok {
			continue
		}
		key := section + "/" + name
		if reachable[key] {
			continue
		}
		reachable[key] = true
		sec, ok := components[section].(map[string]any)
		if !ok {
			continue
		}
		if target, ok := sec[name]; ok {
			queue = append(queue, collectRefs(target)...)
		}
	}

	newComponents := map[string]any{}
	for section, itemsAny := range components {
		if section == "securitySchemes" {
			continue
		}
		items, ok := itemsAny.(map[string]any)
		if !ok {
			continue
		}
		kept := map[string]any{}
		for name, val := range items {
			if reachable[section+"/"+name] {
				kept[name] = val
			}
		}
		if len(kept) > 0 {
			newComponents[section] = kept
		}
	}
	if schemes, ok := components["securitySchemes"].(map[string]any); ok {
		kept := map[string]any{}
		for _, name := range collectSecurityNames(paths, doc["security"]) {
			if val, ok := schemes[name]; ok {
				kept[name] = val
			}
		}
		if len(kept) > 0 {
			newComponents["securitySchemes"] = kept
		}
	}

	if len(newComponents) > 0 {
		doc["components"] = newComponents
	} else {
		delete(doc, "components")
	}
}

// collectRefs рекурсивно собирает все значения "$ref" из узла документа.
func collectRefs(node any) []string {
	var refs []string
	switch v := node.(type) {
	case map[string]any:
		for key, val := range v {
			if key == "$ref" {
				if s, ok := val.(string); ok {
					refs = append(refs, s)
				}
				continue
			}
			refs = append(refs, collectRefs(val)...)
		}
	case []any:
		for _, item := range v {
			refs = append(refs, collectRefs(item)...)
		}
	}
	return refs
}

// collectSecurityNames собирает имена security-схем, на которые ссылаются блоки
// security в путях и в корне документа.
func collectSecurityNames(paths map[string]any, rootSecurity any) []string {
	names := securityNamesFrom(walkSecurity(paths))
	names = append(names, securityNamesFrom(rootSecurity)...)
	return names
}

func walkSecurity(node any) []any {
	var blocks []any
	switch v := node.(type) {
	case map[string]any:
		for key, val := range v {
			if key == "security" {
				blocks = append(blocks, val)
				continue
			}
			blocks = append(blocks, walkSecurity(val)...)
		}
	case []any:
		for _, item := range v {
			blocks = append(blocks, walkSecurity(item)...)
		}
	}
	return blocks
}

func securityNamesFrom(blocks any) []string {
	var names []string
	for _, blockAny := range toSlice(blocks) {
		arr, ok := blockAny.([]any)
		if !ok {
			continue
		}
		for _, reqAny := range arr {
			if req, ok := reqAny.(map[string]any); ok {
				for name := range req {
					names = append(names, name)
				}
			}
		}
	}
	return names
}

func toSlice(v any) []any {
	if s, ok := v.([]any); ok {
		return s
	}
	if v == nil {
		return nil
	}
	return []any{v}
}

// parseComponentRef разбирает "#/components/<section>/<name>" в section и name.
func parseComponentRef(ref string) (section, name string, ok bool) {
	const prefix = "#/components/"
	if !strings.HasPrefix(ref, prefix) {
		return "", "", false
	}
	parts := strings.SplitN(ref[len(prefix):], "/", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", "", false
	}
	name = strings.ReplaceAll(parts[1], "~1", "/")
	name = strings.ReplaceAll(name, "~0", "~")
	return parts[0], name, true
}

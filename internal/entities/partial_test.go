package entities

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// contract — контракт с двумя путями, тремя операциями и схемами, часть которых
// нужна только одной из операций (Order → OrderItem; User — только листингу).
const contract = `{
  "openapi": "3.1.0",
  "info": {"title": "shop", "version": "1.0.0"},
  "paths": {
    "/orders": {
      "post": {
        "operationId": "create-order",
        "requestBody": {"content": {"application/json": {"schema": {"$ref": "#/components/schemas/Order"}}}},
        "responses": {"201": {"description": "ok"}}
      },
      "get": {
        "operationId": "list-orders",
        "responses": {"200": {"description": "ok"}}
      }
    },
    "/users": {
      "get": {
        "operationId": "list-users",
        "responses": {"200": {"content": {"application/json": {"schema": {"$ref": "#/components/schemas/User"}}}}}
      }
    }
  },
  "components": {
    "schemas": {
      "Order": {"type": "object", "properties": {"items": {"type": "array", "items": {"$ref": "#/components/schemas/OrderItem"}}}},
      "OrderItem": {"type": "object", "properties": {"sku": {"type": "string"}}},
      "User": {"type": "object", "properties": {"email": {"type": "string"}}}
    }
  }
}`

func parse(t *testing.T, doc []byte) map[string]any {
	t.Helper()
	var m map[string]any
	require.NoError(t, json.Unmarshal(doc, &m))
	return m
}

func TestSelectMethods_KeepsOnlySelectedOperations(t *testing.T) {
	p := &Protocol{ServiceName: "shop", Document: []byte(contract)}

	got, err := p.SelectMethods([]string{"create-order"})
	require.NoError(t, err)

	m := parse(t, got.Document)
	paths := m["paths"].(map[string]any)
	require.Contains(t, paths, "/orders")
	assert.NotContains(t, paths, "/users", "путь без оставленных операций выкидывается")

	orders := paths["/orders"].(map[string]any)
	assert.Contains(t, orders, "post")
	assert.NotContains(t, orders, "get", "невыбранная операция того же пути выкидывается")
}

func TestSelectMethods_KeepsTransitivelyReferencedSchemas(t *testing.T) {
	p := &Protocol{ServiceName: "shop", Document: []byte(contract)}

	got, err := p.SelectMethods([]string{"create-order"})
	require.NoError(t, err)

	schemas := parse(t, got.Document)["components"].(map[string]any)["schemas"].(map[string]any)
	assert.Contains(t, schemas, "Order")
	assert.Contains(t, schemas, "OrderItem", "вложенная по $ref схема остаётся — срез самодостаточен")
	assert.NotContains(t, schemas, "User", "не нужная срезу схема выкидывается")
}

func TestSelectMethods_ResultStaysValidContract(t *testing.T) {
	p := &Protocol{ServiceName: "shop", Document: []byte(contract)}

	got, err := p.SelectMethods([]string{"list-orders"})
	require.NoError(t, err)
	assert.NoError(t, got.Validate())
}

func TestSelectMethods_Reproducible(t *testing.T) {
	p := &Protocol{ServiceName: "shop", Document: []byte(contract)}

	first, err := p.SelectMethods([]string{"create-order", "list-orders"})
	require.NoError(t, err)
	second, err := p.SelectMethods([]string{"list-orders", "create-order"})
	require.NoError(t, err)

	assert.Equal(t, string(first.Document), string(second.Document),
		"один и тот же набор методов даёт байт-в-байт один и тот же срез")
}

func TestSelectMethods_UnknownMethod_Errors(t *testing.T) {
	p := &Protocol{ServiceName: "shop", Document: []byte(contract)}

	_, err := p.SelectMethods([]string{"create-order", "delete-order"})

	var unknown *UnknownMethodsError
	require.ErrorAs(t, err, &unknown)
	assert.Equal(t, []string{"delete-order"}, unknown.Methods)
}

func TestSelectMethods_Empty_Errors(t *testing.T) {
	p := &Protocol{ServiceName: "shop", Document: []byte(contract)}

	_, err := p.SelectMethods([]string{"  ", ""})
	assert.ErrorIs(t, err, ErrNoMethodsSelected)
}

func TestSelectMethods_DropsComponentsWhenNothingReferenced(t *testing.T) {
	p := &Protocol{ServiceName: "shop", Document: []byte(contract)}

	got, err := p.SelectMethods([]string{"list-orders"})
	require.NoError(t, err)
	assert.NotContains(t, parse(t, got.Document), "components",
		"если срез ни на что не ссылается, components исчезает целиком")
}

func TestSelectMethods_KeepsSecuritySchemesByName(t *testing.T) {
	const secured = `{
      "openapi": "3.1.0",
      "paths": {"/p": {"get": {"operationId": "op", "security": [{"bearer": []}], "responses": {"200": {"description": "ok"}}}}},
      "components": {"securitySchemes": {"bearer": {"type": "http", "scheme": "bearer"}, "unused": {"type": "apiKey", "name": "x", "in": "header"}}}
    }`
	p := &Protocol{ServiceName: "s", Document: []byte(secured)}

	got, err := p.SelectMethods([]string{"op"})
	require.NoError(t, err)

	schemes := parse(t, got.Document)["components"].(map[string]any)["securitySchemes"].(map[string]any)
	assert.Contains(t, schemes, "bearer")
	assert.NotContains(t, schemes, "unused")
}

// Package browserauthorizerlocal — выходной адаптер браузерного входа (AUTH-22).
// CLI поднимает слушателя на петлевом адресе, открывает в браузере страницу
// подтверждения платформы и ждёт, пока та вернёт выпущенный личный токен на этот
// адрес. Секрет не покидает машину пользователя и не копируется руками.
package browserauthorizerlocal

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"github.com/TraumTech/paas-cli/internal/entities"
)

// waitTimeout ограничивает ожидание подтверждения: закрытую вкладку CLI никак не
// увидит, поэтому висеть бесконечно он не должен.
const waitTimeout = 5 * time.Minute

// callbackPath — единственный путь локального слушателя; всё прочее ему чужое.
const callbackPath = "/callback"

type Authorizer struct {
	// webBaseURL — адрес интерфейса платформы: страница подтверждения живёт там,
	// потому что там уже есть вход пользователя.
	webBaseURL string
	// openBrowser подменяется в тестах; в бою — системная команда открытия ссылки.
	openBrowser func(url string) error
	// announce печатает пользователю адрес — на случай, если браузер открылся не
	// там или не открылся вовсе.
	announce io.Writer
}

func New(webBaseURL string, announce io.Writer) *Authorizer {
	return &Authorizer{
		webBaseURL:  strings.TrimRight(webBaseURL, "/"),
		openBrowser: openBrowser,
		announce:    announce,
	}
}

func (a *Authorizer) Authorize(ctx context.Context) (*entities.Credential, error) {
	state, err := randomState()
	if err != nil {
		return nil, err
	}

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, fmt.Errorf("не удалось открыть локальный адрес для ответа браузера: %w", err)
	}
	defer listener.Close()

	results := make(chan result, 1)
	server := &http.Server{Handler: a.callbackHandler(state, results)}
	go func() { _ = server.Serve(listener) }()
	defer func() {
		shutdownCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), time.Second)
		defer cancel()
		_ = server.Shutdown(shutdownCtx)
	}()

	callback := fmt.Sprintf("http://%s%s", listener.Addr().String(), callbackPath)
	authorizeURL := a.authorizeURL(callback, state)
	fmt.Fprintf(a.announce, "Открываем браузер для подтверждения входа:\n%s\n", authorizeURL)
	if err := a.openBrowser(authorizeURL); err != nil {
		return nil, entities.ErrBrowserUnavailable
	}

	waitCtx, cancel := context.WithTimeout(ctx, waitTimeout)
	defer cancel()
	select {
	case res := <-results:
		if res.err != nil {
			return nil, res.err
		}
		return res.credential, nil
	case <-waitCtx.Done():
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		return nil, entities.ErrAuthorizationTimeout
	}
}

type result struct {
	credential *entities.Credential
	err        error
}

func (a *Authorizer) authorizeURL(callback, state string) string {
	query := url.Values{}
	query.Set("callback", callback)
	query.Set("state", state)
	query.Set("label", machineLabel())
	return a.webBaseURL + "/cli/authorize?" + query.Encode()
}

func (a *Authorizer) callbackHandler(state string, results chan<- result) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc(callbackPath, func(w http.ResponseWriter, r *http.Request) {
		query := r.URL.Query()
		// Чужой ответ (другой запуск CLI, подложенная ссылка) не принимаем и о
		// нём не отчитываемся — ждём свой.
		if query.Get("state") != state {
			http.Error(w, "неизвестный запрос", http.StatusForbidden)
			return
		}

		credential, err := parseCallback(query)
		select {
		case results <- result{credential: credential, err: err}:
		default: // ответ уже получен — второй игнорируем
		}

		if err != nil {
			writePage(w, "Вход не подтверждён", "Вернитесь в терминал — CLI сообщит подробности.")
			return
		}
		writePage(w, "Готово", "Вход выполнен. Можно вернуться в терминал.")
	})
	return mux
}

func parseCallback(query url.Values) (*entities.Credential, error) {
	if query.Get("error") != "" {
		return nil, entities.ErrAuthorizationDenied
	}
	token := query.Get("token")
	if token == "" {
		return nil, entities.ErrAuthorizationDenied
	}
	credential := &entities.Credential{
		Kind:    entities.CredentialPersonalToken,
		Token:   token,
		TokenID: query.Get("token_id"),
		Email:   query.Get("email"),
	}
	if expiresAt := query.Get("expires_at"); expiresAt != "" {
		parsed, err := time.Parse(time.RFC3339, expiresAt)
		if err != nil {
			return nil, fmt.Errorf("разбор срока действия токена: %w", err)
		}
		credential.ExpiresAt = parsed
	}
	return credential, nil
}

func writePage(w http.ResponseWriter, title, message string) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	fmt.Fprintf(w, `<!doctype html><meta charset="utf-8"><title>%s — paas-cli</title>`+
		`<body style="font-family:system-ui;margin:4rem auto;max-width:30rem;text-align:center">`+
		`<h1>%s</h1><p>%s</p></body>`, title, title, message)
}

func randomState() (string, error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", fmt.Errorf("подготовка запроса входа: %w", err)
	}
	return hex.EncodeToString(raw), nil
}

// machineLabel — как назвать машину человеку в браузере: по имени хоста понятно,
// с какого компьютера пришёл запрос.
func machineLabel() string {
	host, err := os.Hostname()
	if err != nil || host == "" {
		return "неизвестная машина"
	}
	return host
}

func openBrowser(target string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", target)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", target)
	default:
		cmd = exec.Command("xdg-open", target)
	}
	if err := cmd.Start(); err != nil {
		return errors.New("не удалось открыть браузер")
	}
	// Процесс браузера переживает CLI — ждать его завершения нельзя, но и
	// зомби оставлять незачем.
	go func() { _ = cmd.Wait() }()
	return nil
}

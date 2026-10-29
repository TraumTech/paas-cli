// Package browserauthorizerlocal — выходной адаптер браузерного входа (IAP-03):
// CLI поднимает слушателя на петлевом адресе, открывает в браузере вход
// платформы (сервер авторизации → провайдер входа) и ждёт, пока браузер вернёт
// код выдачи на этот адрес; код тут же меняется на токены. Секрет не покидает
// машину пользователя и не копируется руками.
package browserauthorizerlocal

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os/exec"
	"runtime"
	"time"

	"github.com/TraumTech/paas-cli/internal/entities"
)

// waitTimeout ограничивает ожидание: закрытую вкладку CLI никак не увидит,
// поэтому висеть бесконечно он не должен.
const waitTimeout = 5 * time.Minute

// callbackPath — единственный путь локального слушателя; всё прочее ему чужое.
const callbackPath = "/callback"

// OAuthFlow — сервер авторизации глазами входа: куда вести браузер и как
// обменять принесённый им код на токены.
type OAuthFlow interface {
	AuthorizeURL(redirectURI, state, codeChallenge string) string
	Exchange(ctx context.Context, code, codeVerifier, redirectURI string) (*entities.Credential, error)
}

type Authorizer struct {
	flow OAuthFlow
	// openBrowser подменяется в тестах; в бою — системная команда открытия ссылки.
	openBrowser func(url string) error
	// announce печатает пользователю адрес — на случай, если браузер открылся не
	// там или не открылся вовсе.
	announce io.Writer
}

func New(flow OAuthFlow, announce io.Writer) *Authorizer {
	return &Authorizer{flow: flow, openBrowser: openBrowser, announce: announce}
}

func (a *Authorizer) Authorize(ctx context.Context) (*entities.Credential, error) {
	state, err := randomToken()
	if err != nil {
		return nil, err
	}
	verifier, err := randomToken()
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

	redirectURI := fmt.Sprintf("http://%s%s", listener.Addr().String(), callbackPath)
	authorizeURL := a.flow.AuthorizeURL(redirectURI, state, codeChallenge(verifier))
	fmt.Fprintf(a.announce, "Открываем браузер для входа:\n%s\n", authorizeURL)
	if err := a.openBrowser(authorizeURL); err != nil {
		// Код вернётся только на эту машину, поэтому ссылку нужно открыть здесь же.
		fmt.Fprintln(a.announce, "Браузер открыть не удалось — откройте ссылку на этой машине вручную.")
	}

	waitCtx, cancel := context.WithTimeout(ctx, waitTimeout)
	defer cancel()
	select {
	case res := <-results:
		if res.err != nil {
			return nil, res.err
		}
		return a.flow.Exchange(ctx, res.code, verifier, redirectURI)
	case <-waitCtx.Done():
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		return nil, entities.ErrAuthorizationTimeout
	}
}

type result struct {
	code string
	err  error
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

		code, err := parseCallback(query)
		select {
		case results <- result{code: code, err: err}:
		default: // ответ уже получен — второй игнорируем
		}

		if err != nil {
			writePage(w, "Вход не выполнен", "Вернитесь в терминал — CLI сообщит подробности.")
			return
		}
		writePage(w, "Готово", "Вход выполнен. Можно вернуться в терминал.")
	})
	return mux
}

func parseCallback(query url.Values) (string, error) {
	switch failure := query.Get("error"); failure {
	case "":
	case "access_denied":
		return "", entities.ErrAuthorizationDenied
	default:
		return "", fmt.Errorf("сервер авторизации отказал: %s (%s)", failure, query.Get("error_description"))
	}
	code := query.Get("code")
	if code == "" {
		return "", entities.ErrAuthorizationDenied
	}
	return code, nil
}

func writePage(w http.ResponseWriter, title, message string) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	fmt.Fprintf(w, `<!doctype html><meta charset="utf-8"><title>%s — paas-cli</title>`+
		`<body style="font-family:system-ui;margin:4rem auto;max-width:30rem;text-align:center">`+
		`<h1>%s</h1><p>%s</p></body>`, title, title, message)
}

// randomToken — 32 случайных байта в base64url: годится и как state, и как
// PKCE code_verifier (43 символа из разрешённого алфавита).
func randomToken() (string, error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", fmt.Errorf("подготовка запроса входа: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(raw), nil
}

func codeChallenge(verifier string) string {
	sum := sha256.Sum256([]byte(verifier))
	return base64.RawURLEncoding.EncodeToString(sum[:])
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

package platformapi

// oapi-codegen (v2.7.x) в режиме client-only эмитит константы скоупов
// security-схем контракта, но не типы их ключей контекста (генерируются только
// вместе с серверным кодом). Доопределяем типы здесь; файл рукописный.
type (
	kratosSessionContextKey string
	serviceTokenContextKey  string
)

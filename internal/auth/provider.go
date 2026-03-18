package auth

// Token represents an OAuth access token.
type Token struct {
	AccessToken string
	TokenType   string
	Scope       string
}

// User represents an authenticated user.
type User struct {
	Username string
	Email    string
	ID       int64
}

// Provider defines the authentication interface.
// Implementations exist per git provider (GitHub, GitLab, etc.).
type Provider interface {
	Login() (*Token, error)
	GetUser(accessToken string) (*User, error)
}

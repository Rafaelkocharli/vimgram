package telegram

import (
	"context"
	"errors"

	"github.com/gotd/td/telegram/auth"
	"github.com/gotd/td/tg"
)

// AuthPrompter is implemented by the UI layer to ask the user for sign-in
// credentials. All methods block until the user provides input.
type AuthPrompter interface {
	Phone(ctx context.Context) (string, error)
	Code(ctx context.Context) (string, error)
	Password(ctx context.Context) (string, error)
}

// userAuth adapts our AuthPrompter to gotd's UserAuthenticator interface.
type userAuth struct {
	prompter AuthPrompter
}

func (a userAuth) Phone(ctx context.Context) (string, error) {
	return a.prompter.Phone(ctx)
}

func (a userAuth) Code(ctx context.Context, _ *tg.AuthSentCode) (string, error) {
	return a.prompter.Code(ctx)
}

func (a userAuth) Password(ctx context.Context) (string, error) {
	return a.prompter.Password(ctx)
}

func (userAuth) AcceptTermsOfService(context.Context, tg.HelpTermsOfService) error {
	return nil
}

func (userAuth) SignUp(context.Context) (auth.UserInfo, error) {
	return auth.UserInfo{}, errors.New("sign up not supported, use an existing account")
}

package ui

import (
	"context"
	"fmt"

	tea "github.com/charmbracelet/bubbletea"

	"vimgram/internal/config"
	"vimgram/internal/storage"
	"vimgram/internal/telegram"
)

// Run boots the application: constructs the telegram client, wires the
// bubble-tea program to it, and blocks until the user quits.
func Run(cfg config.Config) error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	session := storage.NewFileSession(cfg.SessionPath)
	client := telegram.NewClient(cfg.AppID, cfg.AppHash, session)

	answers := make(chan string, 1)
	model := NewModel(client, cancel, answers)

	program := tea.NewProgram(model, tea.WithAltScreen())

	// Wire the auth prompter: prompts post a tea message, then block on a
	// channel that the model fills in when the user submits.
	client.SetPrompter(&teaPrompter{program: program, answers: answers})

	// Wire telegram events to bubbletea messages.
	client.SetEventSink(func(e telegram.Event) {
		program.Send(telegramEventMsg{Event: e})
	})

	go func() {
		if err := client.Run(ctx); err != nil && ctx.Err() == nil {
			program.Send(telegramEventMsg{Event: telegram.EventError{Err: err}})
		}
	}()

	if _, err := program.Run(); err != nil {
		return fmt.Errorf("tea program: %w", err)
	}
	return nil
}

// teaPrompter implements telegram.AuthPrompter on top of a tea.Program.
type teaPrompter struct {
	program *tea.Program
	answers chan string
}

func (p *teaPrompter) Phone(ctx context.Context) (string, error) {
	return p.ask(ctx, needPhoneMsg{})
}

func (p *teaPrompter) Code(ctx context.Context) (string, error) {
	return p.ask(ctx, needCodeMsg{})
}

func (p *teaPrompter) Password(ctx context.Context) (string, error) {
	return p.ask(ctx, needPasswordMsg{})
}

func (p *teaPrompter) ask(ctx context.Context, prompt tea.Msg) (string, error) {
	p.program.Send(prompt)
	select {
	case v := <-p.answers:
		return v, nil
	case <-ctx.Done():
		return "", ctx.Err()
	}
}

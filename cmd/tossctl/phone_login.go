package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/JungHoonGhae/tossinvest-cli/internal/auth"
	"github.com/JungHoonGhae/tossinvest-cli/internal/i18n"
	"github.com/JungHoonGhae/tossinvest-cli/internal/tui"
)

// Phone login identity: the same three fields the Toss sign-in form asks for.
// Agents and scripts pass them through the environment; nothing is saved.
const (
	envLoginName  = "TOSSCTL_LOGIN_NAME"
	envLoginBirth = "TOSSCTL_LOGIN_BIRTH"
	envLoginPhone = "TOSSCTL_LOGIN_PHONE"
)

var (
	birthPattern = regexp.MustCompile(`^[0-9]{6}$`)
	phonePattern = regexp.MustCompile(`^01[0-9]{8,9}$`)
)

func validateLoginName(v string) error {
	if strings.TrimSpace(v) == "" {
		return errors.New(i18n.T("auth.phone.err.name"))
	}
	return nil
}

func validateLoginBirth(v string) error {
	v = strings.TrimSpace(v)
	if !birthPattern.MatchString(v) {
		return errors.New(i18n.T("auth.phone.err.birth"))
	}
	// YYMMDD: reject impossible dates (the form would, after a round trip).
	if _, err := time.Parse("060102", v); err != nil {
		return errors.New(i18n.T("auth.phone.err.birth"))
	}
	return nil
}

func normalizePhone(v string) string {
	return strings.Map(func(r rune) rune {
		if r >= '0' && r <= '9' {
			return r
		}
		return -1
	}, v)
}

func validateLoginPhone(v string) error {
	if !phonePattern.MatchString(normalizePhone(v)) {
		return errors.New(i18n.T("auth.phone.err.number"))
	}
	return nil
}

func newPhoneLogin(name, birth, phone string) (*auth.PhoneLogin, error) {
	for _, check := range []error{validateLoginName(name), validateLoginBirth(birth), validateLoginPhone(phone)} {
		if check != nil {
			return nil, check
		}
	}
	return &auth.PhoneLogin{
		Name:   strings.TrimSpace(name),
		Birth:  strings.TrimSpace(birth),
		Number: normalizePhone(phone),
	}, nil
}

// resolvePhoneLogin asks for the login identity on a terminal (after showing
// what is sent where and which terms are agreed) or reads it from the
// environment elsewhere. ok=false means the user declined: nothing was sent.
func resolvePhoneLogin(out io.Writer, getenv func(string) string) (login *auth.PhoneLogin, ok bool, err error) {
	if !tui.IsInteractive(os.Stdin, os.Stdout) {
		name, birth, phone := getenv(envLoginName), getenv(envLoginBirth), getenv(envLoginPhone)
		if name == "" || birth == "" || phone == "" {
			return nil, false, errors.New(i18n.T("auth.phone.err.nonInteractive"))
		}
		login, err := newPhoneLogin(name, birth, phone)
		return login, err == nil, err
	}

	fmt.Fprint(out, i18n.T("auth.phone.intro"))
	name, err := tui.Input(i18n.T("auth.phone.prompt.name"), "", validateLoginName)
	if err != nil {
		return nil, false, err
	}
	birth, err := tui.Input(i18n.T("auth.phone.prompt.birth"), i18n.T("auth.phone.placeholder.birth"), validateLoginBirth)
	if err != nil {
		return nil, false, err
	}
	phone, err := tui.Input(i18n.T("auth.phone.prompt.number"), "01012345678", validateLoginPhone)
	if err != nil {
		return nil, false, err
	}
	agree, err := tui.Confirm(i18n.T("auth.phone.prompt.consent"))
	if err != nil {
		return nil, false, err
	}
	if !agree {
		fmt.Fprint(out, i18n.T("auth.phone.msg.cancelled"))
		return nil, false, nil
	}
	login, err = newPhoneLogin(name, birth, phone)
	return login, err == nil, err
}

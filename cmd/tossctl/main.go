package main

import (
	"fmt"
	"os"

	"github.com/JungHoonGhae/tossinvest-cli/internal/i18n"
)

func main() {
	i18n.SetLang(i18n.Resolve(os.Args[1:], os.Getenv))

	if err := newRootCmd().Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

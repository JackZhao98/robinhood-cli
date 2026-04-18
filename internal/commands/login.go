package commands

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"strings"
	"syscall"

	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/jackzhao/robinhood-cli/internal/auth"
	"github.com/jackzhao/robinhood-cli/internal/output"
)

type cliPrompter struct{}

func (cliPrompter) PromptMFACode(mfaType string) (string, error) {
	fmt.Fprintf(os.Stderr, "MFA required (%s). Enter code: ", mfaType)
	reader := bufio.NewReader(os.Stdin)
	line, err := reader.ReadString('\n')
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(line), nil
}

func (cliPrompter) NotifySheriff(message string) error {
	fmt.Fprintln(os.Stderr, message)
	reader := bufio.NewReader(os.Stdin)
	_, err := reader.ReadString('\n')
	return err
}

func newLoginCmd() *cobra.Command {
	var username, password string
	cmd := &cobra.Command{
		Use:   "login",
		Short: "Log in to Robinhood (pure HTTP, stores tokens locally)",
		RunE: func(cmd *cobra.Command, args []string) error {
			if username == "" {
				if v := os.Getenv("ROBINHOOD_USERNAME"); v != "" {
					username = v
				} else {
					fmt.Fprint(os.Stderr, "Username: ")
					line, err := bufio.NewReader(os.Stdin).ReadString('\n')
					if err != nil {
						return err
					}
					username = strings.TrimSpace(line)
				}
			}
			if password == "" {
				if v := os.Getenv("ROBINHOOD_PASSWORD"); v != "" {
					password = v
				} else {
					fmt.Fprint(os.Stderr, "Password: ")
					pw, err := term.ReadPassword(int(syscall.Stdin))
					if err != nil {
						return err
					}
					fmt.Fprintln(os.Stderr)
					password = string(pw)
				}
			}
			if username == "" || password == "" {
				return errors.New("username and password are required")
			}

			creds, err := auth.Login(username, password, cliPrompter{})
			if err != nil {
				return err
			}
			return output.Emit(map[string]any{
				"status":      "ok",
				"expires_at":  creds.ExpiresAt,
				"device_token": creds.DeviceToken,
			})
		},
	}
	cmd.Flags().StringVarP(&username, "username", "u", "", "Robinhood username (or $ROBINHOOD_USERNAME)")
	cmd.Flags().StringVarP(&password, "password", "p", "", "Robinhood password (or $ROBINHOOD_PASSWORD)")
	return cmd
}

func newLogoutCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "logout",
		Short: "Forget stored credentials",
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := auth.Clear(); err != nil {
				return err
			}
			return output.Emit(map[string]string{"status": "ok"})
		},
	}
}

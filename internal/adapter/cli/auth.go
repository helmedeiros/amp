package cli

import (
	"bufio"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/helmedeiros/amp/internal/adapter/applemusic"
)

// authCmd groups credential setup. Today it holds only Apple Music.
func authCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "auth",
		Short: "Configure external service credentials",
	}
	cmd.AddCommand(authAppleMusicCmd())
	return cmd
}

// authAppleMusicCmd captures the user's media-user-token so amp can add albums
// that streaming left out of the local library. See docs: the token is read
// from the Apple Music web player and lasts about 180 days.
func authAppleMusicCmd() *cobra.Command {
	var token string
	cmd := &cobra.Command{
		Use:   "apple-music",
		Short: "Store your Apple Music token so amp can add missing album tracks",
		Long: strings.TrimSpace(`
Store your Apple Music token so amp can add tracks that streaming left out of
your local library.

To get the token:
  1. Open https://music.apple.com in a browser and sign in.
  2. Open DevTools → Application → Cookies → https://music.apple.com
  3. Copy the value of the "media-user-token" cookie.

Paste it when prompted (or pass --token). It is stored with owner-only
permissions and grants read/write access to your library, so treat it as a
secret. This reuses the web player's own developer token and is not a
sanctioned Apple integration; the token expires after ~180 days.`),
		RunE: func(cmd *cobra.Command, args []string) error {
			out := cmd.OutOrStdout()
			t := strings.TrimSpace(token)
			if t == "" {
				fmt.Fprint(out, "Paste your media-user-token: ")
				r := bufio.NewReader(cmd.InOrStdin())
				line, _ := r.ReadString('\n')
				t = strings.TrimSpace(line)
			}
			if t == "" {
				return fmt.Errorf("no token provided")
			}

			fmt.Fprintln(out, "Validating…")
			creds, err := applemusic.Authenticate(cmd.Context(), t)
			if err != nil {
				return err
			}
			if err := applemusic.SaveCreds(applemusic.CredsPath(), creds); err != nil {
				return err
			}
			fmt.Fprintf(out, "✓ Apple Music connected (storefront: %s)\n", creds.Storefront)
			fmt.Fprintln(out, "amp will now fill in missing album tracks when you play a partial album.")
			fmt.Fprintln(out, "Make sure Sync Library is on in Music → Settings → General.")
			return nil
		},
	}
	cmd.Flags().StringVar(&token, "token", "", "media-user-token (skips the interactive prompt)")
	cmd.AddCommand(authAppleMusicStatusCmd())
	return cmd
}

// authAppleMusicStatusCmd reports whether Apple Music is connected, verifying the
// stored token against the API.
func authAppleMusicStatusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show whether Apple Music is connected and the token still works",
		RunE: func(cmd *cobra.Command, args []string) error {
			creds, err := applemusic.LoadCreds(applemusic.CredsPath())
			if err != nil {
				return err
			}
			var tokenErr error
			if creds.Valid() {
				tokenErr = applemusic.NewClient(creds).Verify(cmd.Context())
			}
			fmt.Fprintln(cmd.OutOrStdout(), applemusic.StatusMessage(creds, tokenErr))
			return nil
		},
	}
}

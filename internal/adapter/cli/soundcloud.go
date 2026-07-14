package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/helmedeiros/amp/internal/adapter/soundcloud"
	"github.com/helmedeiros/amp/internal/port"
)

// soundcloudCmd groups SoundCloud actions.
func soundcloudCmd(ctrl port.Controller) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "soundcloud",
		Short: "Import tracks from SoundCloud into your library",
	}
	cmd.AddCommand(soundcloudImportCmd(ctrl))
	return cmd
}

// soundcloudImportCmd downloads a profile/set/track and imports it into Music.
func soundcloudImportCmd(ctrl port.Controller) *cobra.Command {
	var playlist, soloArtist, dir string
	cmd := &cobra.Command{
		Use:   "import <url>",
		Short: "Download and import SoundCloud tracks (your own uploads)",
		Long: "Download the tracks at a SoundCloud profile, set, or track URL, tag them " +
			"(a \"Band - Song\" title is credited to the band; others to --solo-artist), " +
			"and add them to your library and a playlist. Re-running skips tracks already " +
			"in the library. Intended for your own uploads. Requires yt-dlp and ffmpeg.",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := soundcloud.Available(); err != nil {
				return err
			}
			if dir == "" {
				dir = defaultImportDir()
			}
			out := cmd.OutOrStdout()
			fmt.Fprintln(out, "Importing from SoundCloud… (this downloads audio)")
			res, err := ctrl.ImportSoundCloud(cmd.Context(), args[0], playlist, soloArtist, dir)
			if err != nil {
				return err
			}
			for _, name := range res.Imported {
				fmt.Fprintf(out, "  + %s\n", name)
			}
			fmt.Fprintf(out, "Imported %d, skipped %d (already present), failed %d.\n",
				len(res.Imported), res.Skipped, res.Failed)
			return nil
		},
	}
	cmd.Flags().StringVar(&playlist, "playlist", "SoundCloud", "playlist to add imported tracks to")
	cmd.Flags().StringVar(&soloArtist, "solo-artist", "", "artist for tracks without a \"Band - \" prefix (default: the SoundCloud uploader)")
	cmd.Flags().StringVar(&dir, "dir", "", "where to stage downloaded files (default: ~/Music/amp-soundcloud)")
	return cmd
}

// defaultImportDir returns ~/Music/amp-soundcloud, falling back to the working dir.
func defaultImportDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return "amp-soundcloud"
	}
	return filepath.Join(home, "Music", "amp-soundcloud")
}

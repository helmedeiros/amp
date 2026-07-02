package cli

import (
	"context"
	"fmt"
	"io"
	"os"
	"runtime/debug"
	"strings"

	"github.com/spf13/cobra"

	"github.com/helmedeiros/amp/internal/adapter/tui"
	"github.com/helmedeiros/amp/internal/music"
	"github.com/helmedeiros/amp/internal/port"
)

// NewRootCmd builds the command tree wired to a Controller. Output is written
// to the command's configured streams so it can be captured in tests.
func NewRootCmd(ctrl port.Controller) *cobra.Command {
	var noColor bool

	root := &cobra.Command{
		Use:           "amp",
		Short:         "Control Apple Music from the terminal",
		Version:       buildVersion(),
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	root.PersistentFlags().BoolVar(&noColor, "no-color", false, "disable colored output")

	root.AddCommand(
		statusCmd(ctrl, &noColor),
		nowCmd(ctrl),
		searchCmd(ctrl),
		queueCmd(ctrl),
		playlistsCmd(ctrl),
		libraryCmd(ctrl),
		transportCmd(ctrl, "open", "Launch Apple Music", port.Controller.Open),
		playCmd(ctrl),
		transportCmd(ctrl, "pause", "Pause playback", port.Controller.Pause),
		transportCmd(ctrl, "toggle", "Toggle play/pause", port.Controller.Toggle),
		transportCmd(ctrl, "stop", "Stop playback", port.Controller.Stop),
		transportCmd(ctrl, "next", "Skip to the next track", port.Controller.Next),
		transportCmd(ctrl, "prev", "Skip to the previous track", port.Controller.Previous),
		volCmd(ctrl),
		seekCmd(ctrl),
		muteCmd(ctrl),
		unmuteCmd(ctrl),
		shuffleCmd(ctrl),
		repeatCmd(ctrl),
	)

	return root
}

// buildVersion reports the module version this binary was built from, falling
// back to "dev" for local builds without version metadata.
func buildVersion() string {
	if info, ok := debug.ReadBuildInfo(); ok && info.Main.Version != "" {
		return info.Main.Version
	}
	return "dev"
}

func nowCmd(ctrl port.Controller) *cobra.Command {
	return &cobra.Command{
		Use:   "now",
		Short: "Print the current track on one line",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			s, err := ctrl.Status(cmd.Context())
			if err != nil {
				return err
			}
			fmt.Fprintln(cmd.OutOrStdout(), RenderNow(s))
			return nil
		},
	}
}

func playCmd(ctrl port.Controller) *cobra.Command {
	var limit int

	cmd := &cobra.Command{
		Use:   "play [query]",
		Short: "Resume playback, or play a playlist, album, or search match",
		RunE: func(cmd *cobra.Command, args []string) error {
			res, err := ctrl.PlayQuery(cmd.Context(), strings.Join(args, " "), limit)
			if err != nil {
				return err
			}

			out := cmd.OutOrStdout()
			switch res.Kind {
			case "playlist":
				fmt.Fprintf(out, "▶ playlist: %s\n", res.Label)
			case "album":
				fmt.Fprintf(out, "▶ album: %s\n", res.Label)
			case "track":
				fmt.Fprintf(out, "▶ %s\n", res.Label)
			}
			return nil
		},
		ValidArgsFunction: playCompletions(ctrl),
	}
	cmd.Flags().IntVar(&limit, "limit", 50, "maximum track-search results when the query is not a playlist or album")

	return cmd
}

// playCompletions offers playlist and album names for `play <TAB>`, fetched
// live from the library.
func playCompletions(ctrl port.Controller) func(*cobra.Command, []string, string) ([]string, cobra.ShellCompDirective) {
	return func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		if len(args) > 0 { // the query is a single (possibly quoted) argument
			return nil, cobra.ShellCompDirectiveNoFileComp
		}

		var names []string
		if pls, err := ctrl.Playlists(cmd.Context()); err == nil {
			for _, p := range pls {
				names = append(names, p.Name)
			}
		}
		if albums, err := ctrl.Albums(cmd.Context()); err == nil {
			names = append(names, albums...)
		}

		lower := strings.ToLower(toComplete)
		matches := names[:0]
		for _, n := range names {
			if strings.HasPrefix(strings.ToLower(n), lower) {
				matches = append(matches, n)
			}
		}
		return matches, cobra.ShellCompDirectiveNoFileComp
	}
}

func searchCmd(ctrl port.Controller) *cobra.Command {
	var (
		limit         int
		asJSON        bool
		noInteractive bool
	)

	cmd := &cobra.Command{
		Use:   "search <query>",
		Short: "Search the library and play a result (interactive on a terminal)",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			query := strings.Join(args, " ")
			tracks, err := ctrl.Search(cmd.Context(), query, limit)
			if err != nil {
				return err
			}

			out := cmd.OutOrStdout()

			// Plain, scriptable output when asked for JSON, told not to be
			// interactive, or not writing to a terminal.
			if asJSON {
				fmt.Fprintln(out, RenderTracksJSON(tracks))
				return nil
			}
			if noInteractive || !isTerminal(out) || len(tracks) == 0 {
				fmt.Fprintln(out, RenderTracks(tracks))
				return nil
			}

			// Interactive: pick a starting track; the whole list becomes the
			// queue, playback starts at the pick, the rest play next.
			items := make([]string, len(tracks))
			for i, t := range tracks {
				items[i] = trackLine(t)
			}
			idx, ok, err := tui.Pick("Search: "+query, items)
			if err != nil {
				return err
			}
			if !ok {
				return nil // cancelled
			}
			if err := ctrl.PlaySearch(cmd.Context(), query, limit, idx); err != nil {
				return err
			}

			fmt.Fprintf(out, "▶ %s\n", trackLine(tracks[idx]))
			return nil
		},
	}
	cmd.Flags().IntVar(&limit, "limit", 50, "maximum results (0 for all)")
	cmd.Flags().BoolVar(&asJSON, "json", false, "output machine-readable JSON")
	cmd.Flags().BoolVar(&noInteractive, "no-interactive", false, "print the list instead of the interactive picker")

	return cmd
}

func libraryCmd(ctrl port.Controller) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "library",
		Short: "Browse the library",
		Args:  cobra.NoArgs,
	}
	cmd.AddCommand(
		namesSubcmd("artists", "List all artists", ctrl.Artists),
		namesSubcmd("albums", "List all albums", ctrl.Albums),
	)
	return cmd
}

// namesSubcmd builds a subcommand that prints a list of names fetched from the
// controller, honoring --json.
func namesSubcmd(use, short string, fetch func(context.Context) ([]string, error)) *cobra.Command {
	var asJSON bool

	cmd := &cobra.Command{
		Use:   use,
		Short: short,
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			names, err := fetch(cmd.Context())
			if err != nil {
				return err
			}
			fmt.Fprintln(cmd.OutOrStdout(), RenderNames(names, asJSON))
			return nil
		},
	}
	cmd.Flags().BoolVar(&asJSON, "json", false, "output machine-readable JSON")

	return cmd
}

func queueCmd(ctrl port.Controller) *cobra.Command {
	var asJSON bool

	cmd := &cobra.Command{
		Use:   "queue",
		Short: "Show the play queue, or add to / clear it",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			tracks, err := ctrl.Queue(cmd.Context())
			if err != nil {
				return err
			}

			out := cmd.OutOrStdout()
			if asJSON {
				fmt.Fprintln(out, RenderTracksJSON(tracks))
				return nil
			}
			if len(tracks) == 0 {
				fmt.Fprintln(out, "queue is empty")
				return nil
			}
			fmt.Fprintln(out, RenderTracks(tracks))
			return nil
		},
	}
	cmd.Flags().BoolVar(&asJSON, "json", false, "output machine-readable JSON")

	cmd.AddCommand(queueAddCmd(ctrl), queueClearCmd(ctrl))
	return cmd
}

func queueAddCmd(ctrl port.Controller) *cobra.Command {
	var limit int

	cmd := &cobra.Command{
		Use:   "add <query>",
		Short: "Append search results to the queue without playing",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			n, err := ctrl.QueueAdd(cmd.Context(), strings.Join(args, " "), limit)
			if err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "added %d to queue\n", n)
			return nil
		},
	}
	cmd.Flags().IntVar(&limit, "limit", 50, "maximum results to add (0 for all)")

	return cmd
}

func queueClearCmd(ctrl port.Controller) *cobra.Command {
	return &cobra.Command{
		Use:   "clear",
		Short: "Empty the queue",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if err := ctrl.QueueClear(cmd.Context()); err != nil {
				return err
			}
			fmt.Fprintln(cmd.OutOrStdout(), "queue cleared")
			return nil
		},
	}
}

func playlistsCmd(ctrl port.Controller) *cobra.Command {
	var asJSON bool

	cmd := &cobra.Command{
		Use:   "playlists",
		Short: "List your playlists",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			playlists, err := ctrl.Playlists(cmd.Context())
			if err != nil {
				return err
			}

			out := cmd.OutOrStdout()
			if asJSON {
				fmt.Fprintln(out, RenderPlaylistsJSON(playlists))
			} else {
				fmt.Fprintln(out, RenderPlaylists(playlists))
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&asJSON, "json", false, "output machine-readable JSON")

	return cmd
}

func seekCmd(ctrl port.Controller) *cobra.Command {
	return &cobra.Command{
		Use:   "seek <seconds|mm:ss|+n|-n|n%>",
		Short: "Move the playback position",
		Args:  cobra.ExactArgs(1),
		// A relative argument like "-10" would otherwise be parsed as flags.
		DisableFlagParsing: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if args[0] == "-h" || args[0] == "--help" {
				return cmd.Help()
			}

			mode, value, err := parseSeekArg(args[0])
			if err != nil {
				return err
			}
			pos, err := ctrl.Seek(cmd.Context(), mode, value)
			if err != nil {
				return err
			}

			fmt.Fprintf(cmd.OutOrStdout(), "position %s\n", FormatClock(pos))
			return nil
		},
	}
}

func muteCmd(ctrl port.Controller) *cobra.Command {
	return &cobra.Command{
		Use:   "mute",
		Short: "Silence playback, remembering the current volume",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if err := ctrl.Mute(cmd.Context()); err != nil {
				return err
			}
			fmt.Fprintln(cmd.OutOrStdout(), "muted")
			return nil
		},
	}
}

func unmuteCmd(ctrl port.Controller) *cobra.Command {
	return &cobra.Command{
		Use:   "unmute",
		Short: "Restore the volume from before muting",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			v, err := ctrl.Unmute(cmd.Context())
			if err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "vol %d%%\n", v.Int())
			return nil
		},
	}
}

func onOff(enabled bool) string {
	if enabled {
		return "on"
	}
	return "off"
}

func statusCmd(ctrl port.Controller, noColor *bool) *cobra.Command {
	var asJSON bool

	cmd := &cobra.Command{
		Use:   "status",
		Short: "Show what is playing",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			s, err := ctrl.Status(cmd.Context())
			if err != nil {
				return err
			}

			out := cmd.OutOrStdout()
			if asJSON {
				fmt.Fprintln(out, RenderStatusJSON(s))
				return nil
			}

			theme := PlainTheme
			if wantsColor(out, *noColor) {
				theme = ColorTheme()
			}
			fmt.Fprintln(out, RenderStatus(s, theme))
			return nil
		},
	}
	cmd.Flags().BoolVar(&asJSON, "json", false, "output machine-readable JSON")

	return cmd
}

// wantsColor reports whether colored output should be emitted: never when
// disabled by --no-color or the NO_COLOR convention, and otherwise only when
// the destination is a terminal.
func wantsColor(w io.Writer, noColor bool) bool {
	if noColor || os.Getenv("NO_COLOR") != "" {
		return false
	}
	return isTerminal(w)
}

// isTerminal reports whether w is a character device (an interactive terminal).
func isTerminal(w io.Writer) bool {
	f, ok := w.(*os.File)
	if !ok {
		return false
	}
	info, err := f.Stat()
	return err == nil && info.Mode()&os.ModeCharDevice != 0
}

// transportCmd builds a no-argument command that invokes a single Controller
// action.
func transportCmd(ctrl port.Controller, use, short string, action func(port.Controller, context.Context) error) *cobra.Command {
	return &cobra.Command{
		Use:   use,
		Short: short,
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return action(ctrl, cmd.Context())
		},
	}
}

func shuffleCmd(ctrl port.Controller) *cobra.Command {
	return &cobra.Command{
		Use:       "shuffle [on|off|toggle]",
		Short:     "Turn shuffle on, off, or toggle it (default: toggle)",
		Args:      cobra.MaximumNArgs(1),
		ValidArgs: []string{"on", "off", "toggle"},
		RunE: func(cmd *cobra.Command, args []string) error {
			action := "toggle"
			if len(args) == 1 {
				action = args[0]
			}

			var (
				enabled bool
				err     error
			)
			switch action {
			case "on":
				enabled, err = true, ctrl.SetShuffle(cmd.Context(), true)
			case "off":
				enabled, err = false, ctrl.SetShuffle(cmd.Context(), false)
			case "toggle":
				enabled, err = ctrl.ToggleShuffle(cmd.Context())
			default:
				return fmt.Errorf("invalid shuffle argument %q: want on, off or toggle", action)
			}
			if err != nil {
				return err
			}

			fmt.Fprintf(cmd.OutOrStdout(), "shuffle %s\n", onOff(enabled))
			return nil
		},
	}
}

func repeatCmd(ctrl port.Controller) *cobra.Command {
	return &cobra.Command{
		Use:       "repeat <off|one|all>",
		Short:     "Set the repeat mode",
		Args:      cobra.ExactArgs(1),
		ValidArgs: []string{"off", "one", "all"},
		RunE: func(cmd *cobra.Command, args []string) error {
			mode, err := music.ParseRepeatMode(args[0])
			if err != nil {
				return err
			}
			if err := ctrl.SetRepeat(cmd.Context(), mode); err != nil {
				return err
			}

			fmt.Fprintf(cmd.OutOrStdout(), "repeat %s\n", mode)
			return nil
		},
	}
}

func volCmd(ctrl port.Controller) *cobra.Command {
	return &cobra.Command{
		Use:   "vol <n|+n|-n|up|down>",
		Short: "Get or set the volume",
		Args:  cobra.ExactArgs(1),
		// A relative argument like "-20" would otherwise be parsed as flags.
		DisableFlagParsing: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if args[0] == "-h" || args[0] == "--help" {
				return cmd.Help()
			}

			relative, value, err := parseVolumeArg(args[0])
			if err != nil {
				return err
			}

			var applied music.Volume
			if relative {
				applied, err = ctrl.AdjustVolume(cmd.Context(), value)
			} else {
				applied, err = ctrl.SetVolume(cmd.Context(), value)
			}
			if err != nil {
				return err
			}

			fmt.Fprintf(cmd.OutOrStdout(), "vol %d%%\n", applied.Int())
			return nil
		},
	}
}

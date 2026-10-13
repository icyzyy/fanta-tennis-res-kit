// Command fanta-tennis-res-kit is the CLI for inspecting, decrypting, and
// repacking FantaTennis .res archives.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/icyzyy/fanta-tennis-res-kit/adapter/aesecb"
	"github.com/icyzyy/fanta-tennis-res-kit/adapter/disk"
	"github.com/icyzyy/fanta-tennis-res-kit/adapter/reszip"
	"github.com/icyzyy/fanta-tennis-res-kit/adapter/texdecoder"
	"github.com/icyzyy/fanta-tennis-res-kit/domain"
	"github.com/icyzyy/fanta-tennis-res-kit/usecase"
)

// errUsage marks driver-level usage mistakes → exit code 2.
var errUsage = errors.New("usage error")

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

// run is the testable entry point.
func run(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		usage(stderr)
		return 2
	}
	var err error
	switch args[0] {
	case "decrypt":
		err = cmdDecrypt(args[1:], stdout, stderr)
	case "encrypt":
		err = cmdEncrypt(args[1:], stdout, stderr)
	case "search":
		err = cmdSearch(args[1:], stdout, stderr)
	case "extract":
		err = cmdExtract(args[1:], stdout, stderr)
	case "tex":
		err = cmdTex(args[1:], stdout, stderr)
	case "help", "-h", "--help":
		usage(stdout)
		return 0
	default:
		fmt.Fprintf(stderr, "unknown command %q\n", args[0])
		usage(stderr)
		return 2
	}
	if err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		fmt.Fprintln(stderr, err)
		if errors.Is(err, errUsage) {
			return 2
		}
		return exitCode(err)
	}
	return 0
}

func usage(w io.Writer) {
	fmt.Fprint(w, `usage: fanta-tennis-res-kit <command> [flags] [args]

commands:
  decrypt  [-out FILE] ARCHIVE ENTRY     decrypt a .set entry to stdout or file
  encrypt  [-out FILE] PLAINFILE         encrypt a plaintext file to .set bytes
  search   [-archive A] [-case-sensitive] [-q] ROOT QUERY
  extract  -dest DIR [-entries a,b] [-decrypted] ARCHIVE
  tex      -out FILE ARCHIVE ENTRY       decode a .tex entry to PNG

flags precede positional args; exit codes: 0 ok, 1 internal, 2 usage,
3 integrity, 4 io/not-found
`)
}

// exitCode maps classified errors to the documented codes.
func exitCode(err error) int {
	switch usecase.Classify(err).Code {
	case usecase.CodeBadQuery, usecase.CodeBadDestination:
		return 2
	case usecase.CodeNotSet, usecase.CodeBadPad, usecase.CodeCipherSize,
		usecase.CodeReplaceFailed, usecase.CodeVerifyFailed,
		usecase.CodeBadTexture, usecase.CodeUnsupportedTexture,
		usecase.CodeBadArchive, usecase.CodeInvalidXML,
		usecase.CodeNotEditable, usecase.CodeNotTexture:
		return 3
	case usecase.CodeNotFound, usecase.CodeIO:
		return 4
	default:
		return 1
	}
}

// parseFlags runs a command FlagSet, normalizing errors: ErrHelp passes
// through (exit 0), everything else becomes a usage error (exit 2).
func parseFlags(fs *flag.FlagSet, args []string) error {
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return err
		}
		return fmt.Errorf("%w: %v", errUsage, err)
	}
	return nil
}

func writeOut(path string, data []byte, stdout io.Writer) error {
	if path == "" {
		if _, err := stdout.Write(data); err != nil {
			return fmt.Errorf("write stdout: %w", err)
		}
		return nil
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}
	return nil
}

func cmdDecrypt(args []string, stdout, stderr io.Writer) error {
	fs := flag.NewFlagSet("decrypt", flag.ContinueOnError)
	fs.SetOutput(stderr)
	out := fs.String("out", "", "write plaintext to file instead of stdout")
	if err := parseFlags(fs, args); err != nil {
		return err
	}
	if fs.NArg() != 2 {
		return fmt.Errorf("decrypt: ARCHIVE ENTRY required: %w", errUsage)
	}
	archive, entry := fs.Arg(0), fs.Arg(1)
	content, err := usecase.NewReadEntry(reszip.New(), aesecb.New(), texdecoder.New()).
		Do(context.Background(), archive, entry)
	if err != nil {
		return fmt.Errorf("decrypt: %w", err)
	}
	if content.Kind != usecase.ContentXML {
		return fmt.Errorf("decrypt: %s: %w", entry, domain.ErrNotSetDoc)
	}
	return writeOut(*out, []byte(content.Text), stdout)
}

func cmdEncrypt(args []string, stdout, stderr io.Writer) error {
	fs := flag.NewFlagSet("encrypt", flag.ContinueOnError)
	fs.SetOutput(stderr)
	out := fs.String("out", "", "write .set bytes to file instead of stdout")
	if err := parseFlags(fs, args); err != nil {
		return err
	}
	if fs.NArg() != 1 {
		return fmt.Errorf("encrypt: PLAINFILE required: %w", errUsage)
	}
	plain, err := os.ReadFile(fs.Arg(0))
	if err != nil {
		return fmt.Errorf("encrypt: read %s: %w", fs.Arg(0), err)
	}
	data, err := aesecb.New().Encrypt(plain)
	if err != nil {
		return fmt.Errorf("encrypt: %w", err)
	}
	return writeOut(*out, data, stdout)
}

func cmdSearch(args []string, stdout, stderr io.Writer) error {
	fs := flag.NewFlagSet("search", flag.ContinueOnError)
	fs.SetOutput(stderr)
	archive := fs.String("archive", "", "limit to one archive")
	caseSensitive := fs.Bool("case-sensitive", false, "match case")
	quiet := fs.Bool("q", false, "suppress progress on stderr")
	if err := parseFlags(fs, args); err != nil {
		return err
	}
	if fs.NArg() != 2 {
		return fmt.Errorf("search: ROOT QUERY required: %w", errUsage)
	}
	sa := usecase.NewSearchAll(reszip.New(), aesecb.New(), disk.New())
	progress := usecase.ProgressFunc(func(ev usecase.ProgressEvent) {
		if !*quiet {
			fmt.Fprintf(stderr, "[%d/%d] %s\n", ev.Done, ev.Total, ev.Path)
		}
	})
	return sa.Do(context.Background(), usecase.SearchRequest{
		Root:          fs.Arg(0),
		Archive:       *archive,
		Query:         fs.Arg(1),
		CaseSensitive: *caseSensitive,
	}, func(h usecase.SearchHit) {
		if h.Warning != "" {
			where := h.Archive
			if h.Entry != "" {
				where += "/" + h.Entry
			}
			fmt.Fprintf(stderr, "warning: %s: %s\n", where, h.Warning)
			return
		}
		fmt.Fprintf(stdout, "%s :: %s : %d\t%s\n", h.Archive, h.Entry, h.Line, h.Text)
	}, progress)
}

func cmdExtract(args []string, stdout, stderr io.Writer) error {
	fs := flag.NewFlagSet("extract", flag.ContinueOnError)
	fs.SetOutput(stderr)
	dest := fs.String("dest", "", "destination directory (required)")
	entries := fs.String("entries", "", "comma-separated entry names (default: whole archive)")
	decrypted := fs.Bool("decrypted", false, "decrypt .set entries on the way out")
	if err := parseFlags(fs, args); err != nil {
		return err
	}
	if fs.NArg() != 1 {
		return fmt.Errorf("extract: ARCHIVE required: %w", errUsage)
	}
	if *dest == "" {
		return fmt.Errorf("extract: -dest is required: %w", errUsage)
	}
	req := usecase.ExtractRequest{
		Archive:   fs.Arg(0),
		Dest:      *dest,
		Decrypted: *decrypted,
	}
	if *entries != "" {
		req.Entries = strings.Split(*entries, ",")
	}
	res, err := usecase.NewExtract(reszip.New(), aesecb.New(), disk.New()).
		Do(context.Background(), req, usecase.ProgressFunc(func(ev usecase.ProgressEvent) {
			fmt.Fprintf(stderr, "[%d/%d] %s\n", ev.Done, ev.Total, ev.Path)
		}))
	if err != nil {
		return fmt.Errorf("extract: %w", err)
	}
	for _, f := range res.Files {
		fmt.Fprintln(stdout, f)
	}
	return nil
}

func cmdTex(args []string, stdout, stderr io.Writer) error {
	fs := flag.NewFlagSet("tex", flag.ContinueOnError)
	fs.SetOutput(stderr)
	out := fs.String("out", "", "output PNG file (required)")
	if err := parseFlags(fs, args); err != nil {
		return err
	}
	if fs.NArg() != 2 {
		return fmt.Errorf("tex: ARCHIVE ENTRY required: %w", errUsage)
	}
	if *out == "" {
		return fmt.Errorf("tex: -out is required: %w", errUsage)
	}
	meta, pngBytes, err := usecase.NewPreviewTexture(reszip.New(), texdecoder.New()).
		Do(context.Background(), fs.Arg(0), fs.Arg(1))
	if err != nil {
		return fmt.Errorf("tex: %w", err)
	}
	if err := os.WriteFile(*out, pngBytes, 0o644); err != nil {
		return fmt.Errorf("tex: write %s: %w", *out, err)
	}
	fmt.Fprintf(stderr, "%s: %dx%d %s\n", *out, meta.Width, meta.Height, meta.Format)
	return nil
}

// Command pbmanifest produces signed update manifests for Indie Studio.
//
//	go run ./cmd/pbmanifest keygen -out release-keys
//	go run ./cmd/pbmanifest manifest -key release-keys/private.b64 \
//	  -version 1.0.0 -channel stable -file build/bin/pixelbench-amd64-installer.exe \
//	  -url https://github.com/<owner>/<repo>/releases/download/v1.0.0/pixelbench-installer.exe \
//	  -notes "首个正式版" -out latest.json
//
// The generated latest.json.sig is a base64 Ed25519 signature over the exact
// bytes of latest.json. Embed public.b64 into the app with
// -ldflags "-X main.UpdatePublicKey=<base64>".
package main

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type manifest struct {
	Version     string `json:"version"`
	Channel     string `json:"channel"`
	PublishedAt string `json:"publishedAt"`
	Notes       string `json:"notes,omitempty"`
	PageURL     string `json:"pageUrl,omitempty"`
	Windows     *asset `json:"windows,omitempty"`
}

type asset struct {
	URL    string `json:"url"`
	SHA256 string `json:"sha256"`
	Size   int64  `json:"size"`
}

func main() {
	if len(os.Args) < 2 {
		usage()
	}
	switch os.Args[1] {
	case "keygen":
		keygen(os.Args[2:])
	case "manifest":
		buildManifest(os.Args[2:])
	default:
		usage()
	}
}

func usage() {
	fmt.Fprintln(os.Stderr, "usage: pbmanifest keygen|manifest [flags]")
	os.Exit(2)
}

func keygen(args []string) {
	fs := flag.NewFlagSet("keygen", flag.ExitOnError)
	out := fs.String("out", "release-keys", "directory for private.b64 and public.b64")
	_ = fs.Parse(args)
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		fail(err)
	}
	if err := os.MkdirAll(*out, 0o755); err != nil {
		fail(err)
	}
	seed := priv.Seed()
	if err := os.WriteFile(filepath.Join(*out, "private.b64"), []byte(base64.StdEncoding.EncodeToString(seed)+"\n"), 0o600); err != nil {
		fail(err)
	}
	if err := os.WriteFile(filepath.Join(*out, "public.b64"), []byte(base64.StdEncoding.EncodeToString(pub)+"\n"), 0o644); err != nil {
		fail(err)
	}
	fmt.Printf("private key: %s\npublic key:  %s\n", filepath.Join(*out, "private.b64"), filepath.Join(*out, "public.b64"))
	fmt.Printf("embed with: -ldflags \"-X main.UpdatePublicKey=%s\"\n", base64.StdEncoding.EncodeToString(pub))
}

func buildManifest(args []string) {
	fs := flag.NewFlagSet("manifest", flag.ExitOnError)
	keyFile := fs.String("key", "", "base64 Ed25519 seed (private.b64)")
	version := fs.String("version", "", "release version, e.g. 1.0.0")
	channel := fs.String("channel", "stable", "stable or dev")
	file := fs.String("file", "", "Windows installer to hash")
	url := fs.String("url", "", "public download URL for the installer")
	notes := fs.String("notes", "", "release notes")
	page := fs.String("page", "", "release page URL")
	out := fs.String("out", "latest.json", "manifest output path")
	_ = fs.Parse(args)
	if *keyFile == "" || *version == "" || *file == "" || *url == "" {
		fs.Usage()
		os.Exit(2)
	}
	if *channel == "beta" {
		*channel = "dev"
	}
	if *channel != "stable" && *channel != "dev" {
		fail(fmt.Errorf("channel must be stable or dev"))
	}
	seed, err := readKey(*keyFile)
	if err != nil {
		fail(err)
	}
	priv := ed25519.NewKeyFromSeed(seed)

	info, err := os.Stat(*file)
	if err != nil {
		fail(err)
	}
	data, err := os.ReadFile(*file)
	if err != nil {
		fail(err)
	}
	sum := sha256.Sum256(data)
	payload := manifest{
		Version:     strings.TrimPrefix(*version, "v"),
		Channel:     *channel,
		PublishedAt: time.Now().UTC().Format(time.RFC3339),
		Notes:       *notes,
		PageURL:     *page,
		Windows: &asset{
			URL:    *url,
			SHA256: hex.EncodeToString(sum[:]),
			Size:   info.Size(),
		},
	}
	raw, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		fail(err)
	}
	raw = append(raw, '\n')
	if err := os.WriteFile(*out, raw, 0o644); err != nil {
		fail(err)
	}
	sig := base64.StdEncoding.EncodeToString(ed25519.Sign(priv, raw))
	if err := os.WriteFile(*out+".sig", []byte(sig+"\n"), 0o644); err != nil {
		fail(err)
	}
	fmt.Printf("wrote %s (%d bytes) and %s.sig\n", *out, len(raw), *out)
}

func readKey(path string) ([]byte, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	text := strings.TrimSpace(string(raw))
	if decoded, err := base64.StdEncoding.DecodeString(text); err == nil && len(decoded) == ed25519.SeedSize {
		return decoded, nil
	}
	if decoded, err := hex.DecodeString(text); err == nil && len(decoded) == ed25519.SeedSize {
		return decoded, nil
	}
	return nil, fmt.Errorf("%s is not a 32-byte Ed25519 seed", path)
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, "pbmanifest:", err)
	os.Exit(1)
}

package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/url"
	"strings"
	"testing"
)

func TestParseArgsPreservesCommandLineMode(t *testing.T) {
	opts, interactive, err := parseArgs([]string{
		"-text", "hello",
		"-logo", "./logo.png",
		"-foreground", "#112233",
		"-level", "h",
	})
	if err != nil {
		t.Fatal(err)
	}
	if interactive {
		t.Fatal("normal arguments must keep command-line mode")
	}
	if opts.Text != "hello" || opts.LogoFile != "./logo.png" || opts.Foreground != "#112233" || opts.Level != "H" {
		t.Fatalf("parsed options = %+v", opts)
	}
}

func TestParseArgsSelectsInteractiveMode(t *testing.T) {
	for _, args := range [][]string{nil, {"-interactive"}} {
		_, interactive, err := parseArgs(args)
		if err != nil {
			t.Fatal(err)
		}
		if !interactive {
			t.Fatalf("args %v must select interactive mode", args)
		}
	}
}

func TestParseArgsAcceptsVersionFlag(t *testing.T) {
	for _, args := range [][]string{{"-version"}, {"-v"}} {
		_, interactive, err := parseArgs(args)
		if err != nil {
			t.Fatal(err)
		}
		if interactive {
			t.Fatal("version flag must not enter interactive mode")
		}
	}
}

func TestPromptOptionsAcceptsDocumentedDefaults(t *testing.T) {
	in := strings.NewReader("hello\n" + strings.Repeat("\n", 13))
	var out bytes.Buffer

	opts, err := promptOptions(in, &out, defaultOptions())
	if err != nil {
		t.Fatal(err)
	}
	want := defaultOptions()
	want.Text = "hello"
	if opts != want {
		t.Fatalf("options = %+v, want %+v", opts, want)
	}
	for _, displayedDefault := range []string{"[1]", "[#000000]", "[#ffffff]", "[2]", "[Q]", "[3]", "[400]", "[beautified-qr.png]"} {
		if !strings.Contains(out.String(), displayedDefault) {
			t.Fatalf("wizard output does not display default %q:\n%s", displayedDefault, out.String())
		}
	}
}

func TestPromptOptionsSupportsLocalAndRemoteImages(t *testing.T) {
	tests := []struct {
		name     string
		choice   string
		value    string
		wantFile string
		wantURL  string
	}{
		{name: "local", choice: "2", value: "./sample-logo.png", wantFile: "./sample-logo.png"},
		{name: "remote", choice: "3", value: "https://example.test/logo.png", wantURL: "https://example.test/logo.png"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			input := "hello\n" + tc.choice + "\n" + tc.value + "\n" + strings.Repeat("\n", 12)
			opts, err := promptOptions(strings.NewReader(input), &bytes.Buffer{}, defaultOptions())
			if err != nil {
				t.Fatal(err)
			}
			if opts.LogoFile != tc.wantFile || opts.LogoURL != tc.wantURL {
				t.Fatalf("image options = file %q, URL %q", opts.LogoFile, opts.LogoURL)
			}
		})
	}
}

func TestPromptOptionsMapsMenusAndCollectsCustomEyeColors(t *testing.T) {
	input := strings.Join([]string{
		"hello", // content
		"1",     // no Logo
		"#112233",
		"#F0F0F0",
		"8", // 大圆点
		"6", // 粗圆形
		"n",
		"#445566",
		"#778899",
		"4",
		"4", // H
		"5",
		"500",
		"custom.png",
		"n",
		"y",
	}, "\n") + "\n"

	opts, err := promptOptions(strings.NewReader(input), &bytes.Buffer{}, defaultOptions())
	if err != nil {
		t.Fatal(err)
	}
	if opts.Dot != "大圆点" || opts.Eye != "粗圆形" {
		t.Fatalf("menu mapping = dot %q, eye %q", opts.Dot, opts.Eye)
	}
	if opts.EyeOuter != "#445566" || opts.EyeInner != "#778899" {
		t.Fatalf("eye colors = %q / %q", opts.EyeOuter, opts.EyeInner)
	}
	if opts.Level != "H" || opts.Margin != 4 || opts.Version != 5 || opts.Size != 500 || opts.Output != "custom.png" || opts.Verify {
		t.Fatalf("custom options = %+v", opts)
	}
}

func TestPromptOptionsRepromptsInvalidMenuAndColor(t *testing.T) {
	input := strings.Join([]string{
		"hello",
		"9", "1", // invalid then no Logo
		"red", "#112233", // invalid then valid foreground
		"",        // background
		"99", "8", // invalid then 大圆点
		"", "", "", "", "", "", "", "", "", // remaining defaults
	}, "\n") + "\n"
	var out bytes.Buffer

	opts, err := promptOptions(strings.NewReader(input), &out, defaultOptions())
	if err != nil {
		t.Fatal(err)
	}
	if opts.Foreground != "#112233" || opts.Dot != "大圆点" {
		t.Fatalf("options = %+v", opts)
	}
	if strings.Count(out.String(), "输入无效") < 3 {
		t.Fatalf("invalid input was not reported and re-prompted:\n%s", out.String())
	}
}

func TestPromptOptionsCanCancelBeforeNetworkWork(t *testing.T) {
	input := "hello\n" + strings.Repeat("\n", 12) + "n\n"
	_, err := promptOptions(strings.NewReader(input), &bytes.Buffer{}, defaultOptions())
	if !errors.Is(err, errCancelled) {
		t.Fatalf("error = %v, want errCancelled", err)
	}
}

func TestPromptOptionsReturnsUsefulEOFError(t *testing.T) {
	_, err := promptOptions(strings.NewReader("hello\n"), &bytes.Buffer{}, defaultOptions())
	if err == nil || !strings.Contains(err.Error(), "输入结束") {
		t.Fatalf("error = %v", err)
	}
}

func TestSaveResponseAcceptsStringQRID(t *testing.T) {
	var got saveResponse
	err := json.Unmarshal([]byte(`{"status":1,"data":{"qrid":"1121629085"}}`), &got)
	if err != nil {
		t.Fatalf("string qrid must be accepted: %v", err)
	}
	if got.Data.QRID.Int64() != 1121629085 {
		t.Fatalf("qrid = %d", got.Data.QRID.Int64())
	}
}

func TestBuildMHConfigMapsHumanReadableOptions(t *testing.T) {
	opts := options{
		Text:       "hello",
		LogoURL:    "https://example.test/logo.png",
		Foreground: "#1E4299",
		Background: "#CED9CD",
		Dot:        "大圆点",
		Eye:        "粗圆形",
		EyeOuter:   "#4A5E60",
		EyeInner:   "#8A2930",
		Margin:     4,
		Level:      "H",
		Version:    3,
		Size:       400,
	}

	cfg, err := buildMHConfig(opts)
	if err != nil {
		t.Fatal(err)
	}

	if cfg.BodyType != "17" {
		t.Fatalf("body_type = %q, want 17", cfg.BodyType)
	}
	if cfg.QRCodeEyes != "circle_circle" {
		t.Fatalf("qrcode_eyes = %q, want circle_circle", cfg.QRCodeEyes)
	}
	if cfg.LogoURL != opts.LogoURL || cfg.Foreground != opts.Foreground {
		t.Fatalf("config lost logo/foreground: %+v", cfg)
	}
	if cfg.Level != "H" || cfg.Version != "3" || cfg.MarginBlock != "4" {
		t.Fatalf("config lost QR settings: %+v", cfg)
	}
}

func TestBuildMHConfigAcceptsResolvedUploadedLogo(t *testing.T) {
	opts := options{
		Text: "hello", LogoFile: "/tmp/logo.png", LogoURL: "https://example.test/logo.png",
		Foreground: "#000000", Background: "#ffffff", Dot: "普通", Eye: "方正",
		Level: "H", Version: 3, Size: 400,
	}
	if _, err := buildMHConfig(opts); err != nil {
		t.Fatalf("resolved upload must be usable: %v", err)
	}
}

func TestBuildPaintFormUsesThemeBGForVisibleBackground(t *testing.T) {
	opts := options{Text: "hello", Background: "#CED9CD"}
	mh := mhConfig{Background: "#ffffff"}
	style := styleTemplate{
		CustomTemplateParamConfig: `{"field":[]}`,
		VariableAssocField:        `{"fields":[]}`,
	}

	form, err := buildPaintForm(opts, style, mh)
	if err != nil {
		t.Fatal(err)
	}

	var theme map[string]string
	if err := json.Unmarshal([]byte(form.Get("theme_bg")), &theme); err != nil {
		t.Fatal(err)
	}
	if theme["color"] != "#CED9CD" {
		t.Fatalf("theme_bg color = %q", theme["color"])
	}
	var gotMH mhConfig
	if err := json.Unmarshal([]byte(form.Get("mh_str")), &gotMH); err != nil {
		t.Fatal(err)
	}
	if gotMH.Background != "#ffffff" {
		t.Fatalf("mh_str bgcolor = %q, want #ffffff", gotMH.Background)
	}
}

func TestValidateOptionsRejectsBadInputs(t *testing.T) {
	tests := []options{
		{Text: "", Foreground: "#000000", Background: "#ffffff", Level: "H", Version: 1, Size: 400},
		{Text: "x", Foreground: "red", Background: "#ffffff", Level: "H", Version: 1, Size: 400},
		{Text: "x", Foreground: "#000000", Background: "#ffffff", Level: "Z", Version: 1, Size: 400},
		{Text: "x", Foreground: "#000000", Background: "#ffffff", Level: "H", Version: 41, Size: 400},
	}
	for i, tc := range tests {
		if err := validateOptions(tc); err == nil {
			t.Fatalf("case %d: expected validation error", i)
		}
	}
}

func TestDotAndEyeMappingsAreComplete(t *testing.T) {
	if len(dotTypes) != 14 {
		t.Fatalf("dotTypes has %d entries, want 14", len(dotTypes))
	}
	if len(eyeTypes) != 13 {
		t.Fatalf("eyeTypes has %d entries, want 13", len(eyeTypes))
	}
	if dotTypes["小方点"] != "32" || eyeTypes["四码眼"] != "four_black_eye" {
		t.Fatal("known enum mappings are missing")
	}
}

func TestBuildSaveFormIncludesPreCreateFlag(t *testing.T) {
	form := buildSaveForm(options{Text: "hello", Level: "Q", Size: 400, Margin: 2, Version: 3})
	want := url.Values{
		"info":          {"hello"},
		"content":       {"hello"},
		"is_pre_create": {"1"},
	}
	for key, values := range want {
		if form.Get(key) != values[0] {
			t.Fatalf("%s = %q, want %q", key, form.Get(key), values[0])
		}
	}
}

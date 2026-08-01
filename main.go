package main

import (
	"bufio"
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"time"
)

const (
	appVersion    = "1.1"
	homeURL       = "https://cli.im/"
	saveURL       = "https://cli.im/Apis/QrCode/saveStatic"
	uploadURL     = "https://upload-api.cli.im/upload?kid=cliim"
	paintURL      = "https://qr.api.cli.im/create/paintLabelByParam"
	detectURL     = "https://qrdetector-api.cli.im/v1/detect_binary"
	defaultUA     = "Mozilla/5.0 (compatible; cliimqr/" + appVersion + ")"
	defaultRefURL = "https://cli.im/text/other"
)

var dotTypes = map[string]string{
	"普通": "0", "液化": "8", "圆液化": "29", "条纹": "30",
	"横条纹": "2", "竖条纹": "3", "瓷砖": "31", "大圆点": "17",
	"小圆点": "22", "粗星形": "20", "细星形": "21", "网格": "25",
	"菱形": "19", "小方点": "32",
}

var eyeTypes = map[string]string{
	"方正": "normal", "圆角": "pin-4.png", "粗圆角": "pin-3.png",
	"中圆角": "new20", "细圆角": "new19", "粗圆形": "circle_circle",
	"细圆形": "new18", "菱形": "circle_rhombic", "星形": "circle_star",
	"气泡": "pin-12.png", "眼睛": "pin-8.png", "单圆角": "one_round",
	"四码眼": "four_black_eye",
}

var dotChoices = []string{
	"普通", "液化", "圆液化", "条纹", "横条纹", "竖条纹", "瓷砖",
	"大圆点", "小圆点", "粗星形", "细星形", "网格", "菱形", "小方点",
}

var eyeChoices = []string{
	"方正", "圆角", "粗圆角", "中圆角", "细圆角", "粗圆形", "细圆形",
	"菱形", "星形", "气泡", "眼睛", "单圆角", "四码眼",
}

var hexColor = regexp.MustCompile(`^#[0-9A-Fa-f]{6}$`)

var errCancelled = errors.New("cancelled")

type options struct {
	Text           string
	LogoFile       string
	LogoURL        string
	Foreground     string
	Background     string
	Dot            string
	Eye            string
	EyeOuter       string
	EyeInner       string
	Margin         int
	Level          string
	Version        int
	Size           int
	Output         string
	Verify         bool
	VersionFlag    bool
}

type mhConfig struct {
	LogoURL       string `json:"logourl"`
	LogoShape     string `json:"logoshape"`
	LogoPosition  string `json:"logo_pos"`
	LogoShadow    int    `json:"logo_shadow"`
	LogoSize      int    `json:"logosize"`
	LogoHeight    string `json:"logoh"`
	Data          string `json:"data"`
	Level         string `json:"level"`
	Transparent   int    `json:"transparent"`
	Background    string `json:"bgcolor"`
	Foreground    string `json:"forecolor"`
	BlockPixel    int    `json:"blockpixel"`
	MarginBlock   string `json:"marginblock"`
	Size          string `json:"size"`
	Version       string `json:"version"`
	ForeType      int    `json:"foretype,omitempty"`
	ForeColor2    string `json:"forecolor2,omitempty"`
	BodyType      string `json:"body_type,omitempty"`
	QRCodeEyes    string `json:"qrcode_eyes,omitempty"`
	OuterEyeColor string `json:"outcolor,omitempty"`
	InnerEyeColor string `json:"incolor,omitempty"`
	EyeUseFore    int    `json:"eye_use_fore"`
}

type styleTemplate struct {
	CustomTemplateParamConfig string
	VariableAssocField        string
	LabelTemplateID           int
	StyleTemplateID           int
	StyleSizeID               int
	FromCaseType              int
	CodeType                  int
}

type flexibleInt64 int64

func (n *flexibleInt64) UnmarshalJSON(data []byte) error {
	s := strings.Trim(string(data), `"`)
	v, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return err
	}
	*n = flexibleInt64(v)
	return nil
}

func (n flexibleInt64) Int64() int64 { return int64(n) }

type saveResponse struct {
	Status int    `json:"status"`
	Info   string `json:"info"`
	Data   struct {
		QRID     flexibleInt64 `json:"qrid"`
		QRWebURL string        `json:"qr_web_url"`
		CodeType int           `json:"code_type"`
		Style    struct {
			TemplateID                int    `json:"tpl_id"`
			StyleTemplateID           int    `json:"style_tplid"`
			StyleSizeID               int    `json:"style_size_id"`
			FromCaseType              int    `json:"from_case_type"`
			CustomTemplateParamConfig string `json:"custom_tpl_param_config"`
			VariableAssocField        string `json:"variable_ass_field"`
		} `json:"style_tpl"`
	} `json:"data"`
}

type paintResponse struct {
	Code int    `json:"code"`
	Msg  string `json:"msg"`
	Data struct {
		QRVersion int    `json:"qr_version"`
		Image     string `json:"img_base64"`
	} `json:"data"`
}

type uploadResponse struct {
	Code int    `json:"code"`
	Msg  string `json:"msg"`
	Data struct {
		URL string `json:"url"`
	} `json:"data"`
}

var mainFlagSet *flag.FlagSet

func main() {
	opts, interactive, err := parseArgs(os.Args[1:])
	if errors.Is(err, flag.ErrHelp) {
		return
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(2)
	}
	if opts.VersionFlag {
		fmt.Printf("cliimqr %s %s/%s\n", appVersion, runtime.GOOS, runtime.GOARCH)
		fmt.Println("草料美化二维码命令行工具")
		return
	}
	if len(os.Args[1:]) == 0 {
		mainFlagSet.Usage()
		return
	}
	if interactive {
		opts, err = promptOptions(os.Stdin, os.Stdout, opts)
		if errors.Is(err, errCancelled) {
			fmt.Println("已取消，未生成二维码。")
			return
		}
		if err != nil {
			fmt.Fprintln(os.Stderr, "error:", err)
			os.Exit(1)
		}
	}
	if err := run(opts); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func defaultOptions() options {
	return options{
		Foreground: "#000000",
		Background: "#ffffff",
		Dot:        "普通",
		Eye:        "方正",
		Margin:     2,
		Level:      "Q",
		Version:    3,
		Size:       400,
		Output:     "beautified-qr.png",
		Verify:     true,
	}
}

func parseArgs(args []string) (options, bool, error) {
	o := defaultOptions()
	interactive := false
	fs := flag.NewFlagSet("cliimqr", flag.ContinueOnError)
	fs.StringVar(&o.Text, "text", o.Text, "二维码编码内容（命令行模式必填）")
	fs.StringVar(&o.LogoFile, "logo", o.LogoFile, "本地 Logo 文件（PNG/JPG/BMP）")
	fs.StringVar(&o.LogoURL, "logo-url", o.LogoURL, "远程 Logo URL；与 -logo 二选一")
	fs.StringVar(&o.Foreground, "foreground", o.Foreground, "码点颜色")
	fs.StringVar(&o.Background, "background", o.Background, "可见背景色")
	fs.StringVar(&o.Dot, "dot", o.Dot, "码点形态，例如 普通/大圆点/小方点")
	fs.StringVar(&o.Eye, "eye", o.Eye, "码眼形状，例如 方正/粗圆形/四码眼")
	fs.StringVar(&o.EyeOuter, "eye-outer", o.EyeOuter, "码外眼颜色；留空跟随码点色")
	fs.StringVar(&o.EyeInner, "eye-inner", o.EyeInner, "码内眼颜色；留空跟随码点色")
	fs.IntVar(&o.Margin, "margin", o.Margin, "码边距（色块数）")
	fs.StringVar(&o.Level, "level", o.Level, "容错率 L/M/Q/H")
	fs.IntVar(&o.Version, "qrversion", o.Version, "二维码版本 1-40")
	fs.IntVar(&o.Size, "size", o.Size, "二维码内部绘制尺寸")
	fs.StringVar(&o.Output, "out", o.Output, "输出 PNG 路径")
	fs.BoolVar(&o.Verify, "verify", o.Verify, "生成后调用检测接口回读内容")
	fs.BoolVar(&o.VersionFlag, "version", false, "显示程序版本信息")
	fs.BoolVar(&o.VersionFlag, "v", false, "显示程序版本信息（简写）")
	fs.BoolVar(&interactive, "interactive", false, "进入终端对话模式")
	fs.Usage = func() { printUsage(fs) }
	mainFlagSet = fs
	if err := fs.Parse(args); err != nil {
		return options{}, false, err
	}
	o.Level = strings.ToUpper(o.Level)
	return o, interactive || len(args) == 0, nil
}

func printUsage(fs *flag.FlagSet) {
	fmt.Fprintf(os.Stderr, `cliimqr %s — 草料美化二维码命令行工具

通过草料二维码网页接口生成美化二维码，支持终端交互向导和命令行两种模式。

========================================
用法
========================================

  cliimqr                         进入终端交互向导（逐步配置）
  cliimqr -text <内容> -out <路径>  快速生成二维码
  cliimqr -h                       显示本帮助

========================================
参数
========================================

`, appVersion)
	fs.VisitAll(func(f *flag.Flag) {
		def := ""
		if f.DefValue != "" && f.Name != "version" && f.Name != "v" && f.Name != "interactive" {
			def = fmt.Sprintf(" (默认: %s)", f.DefValue)
		}
		fmt.Fprintf(os.Stderr, "  -%-12s  %s%s\n", f.Name, f.Usage, def)
	})
	fmt.Fprintf(os.Stderr, `
========================================
码点形态（-dot）
========================================

  普通 / 液化 / 圆液化 / 条纹 / 横条纹 / 竖条纹
  瓷砖 / 大圆点 / 小圆点 / 粗星形 / 细星形
  网格 / 菱形 / 小方点

========================================
码眼形状（-eye）
========================================

  方正 / 圆角 / 粗圆角 / 中圆角 / 细圆角
  粗圆形 / 细圆形 / 菱形 / 星形 / 气泡
  眼睛 / 单圆角 / 四码眼

========================================
示例
========================================

  1. 无 Logo 快速生成

     cliimqr -text 'https://example.com' -out example.png

  2. 使用本地 Logo

     cliimqr -text 'https://example.com' -logo ./logo.png ^
       -foreground '#1268D8' -background '#FFFFFF' ^
       -dot '大圆点' -eye '粗圆形' ^
       -margin 4 -level H -out branded.png

  3. 完整自定义

     cliimqr -text 'https://example.com' -logo ./logo.png ^
       -foreground '#111827' -background '#FFFFFF' ^
       -dot '小方点' -eye '四码眼' ^
       -eye-outer '#0758B8' -eye-inner '#29B6F6' ^
       -margin 4 -level H -qrversion 4 ^
       -size 400 -out custom.png

  4. 查看版本信息

     cliimqr -version
     cliimqr -v

========================================
说明
========================================

  - 输出的最终图片为 500×500 PNG（草料标签预览图尺寸）
  - -size 控制二维码在画布中的内部绘制尺寸，不等于输出像素尺寸
  - 使用 Logo 时建议 -level H -margin 4 以提高容错
  - 默认开启回读校验（-verify=true），确保图片可扫描且内容一致
  - 无参数运行自动进入交互向导
  - 需要网络访问: cli.im / upload-api.cli.im / qr.api.cli.im / qrdetector-api.cli.im

完整文档: https://github.com/hhhaiai/cliimqr

========================================
支持
========================================

  本工具由 SimiRouter 中转站支持

  https://api.dwchainless.com/

  高可用 · 低延迟 · 透明计费
  一个 API 链接全球主流 AI 模型
`)
}

type wizard struct {
	scanner *bufio.Scanner
	out     io.Writer
}

func promptOptions(in io.Reader, out io.Writer, o options) (options, error) {
	w := wizard{scanner: bufio.NewScanner(in), out: out}
	fmt.Fprintln(out, "=== cli.im 美化二维码终端向导 ===")
	fmt.Fprintln(out, "直接按 Enter 接受方括号中的默认值。")

	text, err := w.required("编码内容 [必填]: ")
	if err != nil {
		return options{}, err
	}
	o.Text = text

	fmt.Fprintln(out, "\nLogo / 图片来源：")
	logoChoice, err := w.choice([]string{"不使用 Logo", "使用本地图片", "使用远程图片 URL"}, 1, "请选择图片来源")
	if err != nil {
		return options{}, err
	}
	o.LogoFile, o.LogoURL = "", ""
	switch logoChoice {
	case 2:
		o.LogoFile, err = w.required("本地图片路径 [必填]: ")
	case 3:
		o.LogoURL, err = w.required("远程图片 URL [必填]: ")
	}
	if err != nil {
		return options{}, err
	}

	if o.Foreground, err = w.color("码点颜色", o.Foreground); err != nil {
		return options{}, err
	}
	if o.Background, err = w.color("码背景色", o.Background); err != nil {
		return options{}, err
	}

	fmt.Fprintln(out, "\n码点形态：")
	dotIndex, err := w.choice(dotChoices, choiceIndex(dotChoices, o.Dot), "请选择码点形态")
	if err != nil {
		return options{}, err
	}
	o.Dot = dotChoices[dotIndex-1]

	fmt.Fprintln(out, "\n码眼形状：")
	eyeIndex, err := w.choice(eyeChoices, choiceIndex(eyeChoices, o.Eye), "请选择码眼形状")
	if err != nil {
		return options{}, err
	}
	o.Eye = eyeChoices[eyeIndex-1]

	follow, err := w.yesNo("码眼颜色跟随码点颜色", o.EyeOuter == "" && o.EyeInner == "")
	if err != nil {
		return options{}, err
	}
	if follow {
		o.EyeOuter, o.EyeInner = "", ""
	} else {
		if o.EyeOuter == "" {
			o.EyeOuter = o.Foreground
		}
		if o.EyeInner == "" {
			o.EyeInner = o.Foreground
		}
		if o.EyeOuter, err = w.color("码外眼颜色", o.EyeOuter); err != nil {
			return options{}, err
		}
		if o.EyeInner, err = w.color("码内眼颜色", o.EyeInner); err != nil {
			return options{}, err
		}
	}

	if o.Margin, err = w.integer("码边距（色块数）", o.Margin, 0, 20); err != nil {
		return options{}, err
	}
	fmt.Fprintln(out, "\n容错率：\n  1. L\n  2. M\n  3. Q\n  4. H")
	levelIndex, err := w.level(o.Level)
	if err != nil {
		return options{}, err
	}
	o.Level = []string{"L", "M", "Q", "H"}[levelIndex-1]
	if o.Version, err = w.integer("二维码版本", o.Version, 1, 40); err != nil {
		return options{}, err
	}
	if o.Size, err = w.integer("内部绘制尺寸", o.Size, 100, 2000); err != nil {
		return options{}, err
	}
	if o.Output, err = w.withDefault("输出 PNG 路径", o.Output); err != nil {
		return options{}, err
	}
	if o.Verify, err = w.yesNo("生成后回读校验", o.Verify); err != nil {
		return options{}, err
	}

	printSummary(out, o)
	confirmed, err := w.yesNo("确认生成", true)
	if err != nil {
		return options{}, err
	}
	if !confirmed {
		return options{}, errCancelled
	}
	return o, nil
}

func choiceIndex(choices []string, current string) int {
	for i, value := range choices {
		if value == current {
			return i + 1
		}
	}
	return 1
}

func (w wizard) read(prompt string) (string, error) {
	fmt.Fprint(w.out, prompt)
	if !w.scanner.Scan() {
		if err := w.scanner.Err(); err != nil {
			return "", fmt.Errorf("读取输入: %w", err)
		}
		return "", errors.New("输入结束，终端向导未完成")
	}
	return w.scanner.Text(), nil
}

func (w wizard) required(prompt string) (string, error) {
	for {
		value, err := w.read(prompt)
		if err != nil {
			return "", err
		}
		if strings.TrimSpace(value) != "" {
			return value, nil
		}
		fmt.Fprintln(w.out, "输入无效：此项不能为空。")
	}
}

func (w wizard) withDefault(label, defaultValue string) (string, error) {
	value, err := w.read(fmt.Sprintf("%s [%s]: ", label, defaultValue))
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(value) == "" {
		return defaultValue, nil
	}
	return value, nil
}

func (w wizard) choice(choices []string, defaultChoice int, label string) (int, error) {
	for i, value := range choices {
		fmt.Fprintf(w.out, "  %d. %s\n", i+1, value)
	}
	for {
		value, err := w.read(fmt.Sprintf("%s [%d]: ", label, defaultChoice))
		if err != nil {
			return 0, err
		}
		if strings.TrimSpace(value) == "" {
			return defaultChoice, nil
		}
		selected, parseErr := strconv.Atoi(strings.TrimSpace(value))
		if parseErr == nil && selected >= 1 && selected <= len(choices) {
			return selected, nil
		}
		fmt.Fprintf(w.out, "输入无效：请输入 1-%d。\n", len(choices))
	}
}

func (w wizard) color(label, defaultValue string) (string, error) {
	for {
		value, err := w.withDefault(label, defaultValue)
		if err != nil {
			return "", err
		}
		if hexColor.MatchString(value) {
			return value, nil
		}
		fmt.Fprintln(w.out, "输入无效：颜色必须是 #RRGGBB。")
	}
}

func (w wizard) integer(label string, defaultValue, min, max int) (int, error) {
	for {
		value, err := w.read(fmt.Sprintf("%s [%d]: ", label, defaultValue))
		if err != nil {
			return 0, err
		}
		if strings.TrimSpace(value) == "" {
			return defaultValue, nil
		}
		n, parseErr := strconv.Atoi(strings.TrimSpace(value))
		if parseErr == nil && n >= min && n <= max {
			return n, nil
		}
		fmt.Fprintf(w.out, "输入无效：请输入 %d-%d。\n", min, max)
	}
}

func (w wizard) level(defaultLevel string) (int, error) {
	levels := []string{"L", "M", "Q", "H"}
	defaultChoice := choiceIndex(levels, strings.ToUpper(defaultLevel))
	for {
		value, err := w.read(fmt.Sprintf("请选择容错率 [%s]: ", levels[defaultChoice-1]))
		if err != nil {
			return 0, err
		}
		value = strings.ToUpper(strings.TrimSpace(value))
		if value == "" {
			return defaultChoice, nil
		}
		if n, parseErr := strconv.Atoi(value); parseErr == nil && n >= 1 && n <= len(levels) {
			return n, nil
		}
		if n := choiceIndex(levels, value); levels[n-1] == value {
			return n, nil
		}
		fmt.Fprintln(w.out, "输入无效：请输入 1-4 或 L/M/Q/H。")
	}
}

func (w wizard) yesNo(label string, defaultYes bool) (bool, error) {
	defaultHint := "Y/n"
	if !defaultYes {
		defaultHint = "y/N"
	}
	for {
		value, err := w.read(fmt.Sprintf("%s [%s]: ", label, defaultHint))
		if err != nil {
			return false, err
		}
		switch strings.ToLower(strings.TrimSpace(value)) {
		case "":
			return defaultYes, nil
		case "y", "yes", "是":
			return true, nil
		case "n", "no", "否":
			return false, nil
		default:
			fmt.Fprintln(w.out, "输入无效：请输入 y 或 n。")
		}
	}
}

func printSummary(out io.Writer, o options) {
	logo := "无"
	if o.LogoFile != "" {
		logo = "本地图片：" + o.LogoFile
	} else if o.LogoURL != "" {
		logo = "远程图片：" + o.LogoURL
	}
	eyeColors := "跟随码点颜色"
	if o.EyeOuter != "" || o.EyeInner != "" {
		eyeColors = fmt.Sprintf("外眼 %s / 内眼 %s", o.EyeOuter, o.EyeInner)
	}
	fmt.Fprintf(out, "\n=== 配置确认 ===\n编码内容：%s\nLogo：%s\n码点颜色：%s\n码背景色：%s\n码点形态：%s\n码眼形状：%s\n码眼颜色：%s\n码边距：%d\n容错率：%s\n二维码版本：%d\n内部尺寸：%d\n输出路径：%s\n回读校验：%t\n",
		o.Text, logo, o.Foreground, o.Background, o.Dot, o.Eye, eyeColors,
		o.Margin, o.Level, o.Version, o.Size, o.Output, o.Verify)
}

func run(opts options) error {
	if err := validateOptions(opts); err != nil {
		return err
	}
	jar, err := cookiejar.New(nil)
	if err != nil {
		return err
	}
	client := &http.Client{Jar: jar, Timeout: 45 * time.Second}
	if err := initSession(client); err != nil {
		return fmt.Errorf("初始化会话: %w", err)
	}

	if opts.LogoFile != "" {
		opts.LogoURL, err = uploadLogo(client, opts.LogoFile)
		if err != nil {
			return fmt.Errorf("上传 Logo: %w", err)
		}
		fmt.Println("logo:", opts.LogoURL)
	}

	saved, err := saveStatic(client, opts)
	if err != nil {
		return fmt.Errorf("创建静态码: %w", err)
	}
	mh, err := buildMHConfig(opts)
	if err != nil {
		return err
	}
	style := styleTemplate{
		CustomTemplateParamConfig: saved.Data.Style.CustomTemplateParamConfig,
		VariableAssocField:        saved.Data.Style.VariableAssocField,
		LabelTemplateID:           saved.Data.Style.TemplateID,
		StyleTemplateID:           saved.Data.Style.StyleTemplateID,
		StyleSizeID:               saved.Data.Style.StyleSizeID,
		FromCaseType:              saved.Data.Style.FromCaseType,
		CodeType:                  saved.Data.CodeType,
	}
	form, err := buildPaintForm(opts, style, mh)
	if err != nil {
		return err
	}
	painted, err := paint(client, form)
	if err != nil {
		return fmt.Errorf("绘制美化二维码: %w", err)
	}
	imageBytes, err := decodeDataURL(painted.Data.Image)
	if err != nil {
		return err
	}
	if err := os.WriteFile(opts.Output, imageBytes, 0o644); err != nil {
		return err
	}
	abs, _ := filepath.Abs(opts.Output)
	fmt.Printf("output: %s\nqrid: %d\nversion: %d\n", abs, saved.Data.QRID.Int64(), painted.Data.QRVersion)

	if opts.Verify {
		decoded, err := detect(client, painted.Data.Image)
		if err != nil {
			return fmt.Errorf("生成成功但回读校验失败: %w", err)
		}
		if decoded != opts.Text {
			return fmt.Errorf("回读内容不一致: got %q, want %q", decoded, opts.Text)
		}
		fmt.Println("verified:", decoded)
	}
	return nil
}

func validateOptions(o options) error {
	if strings.TrimSpace(o.Text) == "" {
		return errors.New("-text 不能为空")
	}
	for name, color := range map[string]string{
		"foreground": o.Foreground,
		"background": o.Background,
	} {
		if !hexColor.MatchString(color) {
			return fmt.Errorf("-%s 必须是 #RRGGBB", name)
		}
	}
	for name, color := range map[string]string{"eye-outer": o.EyeOuter, "eye-inner": o.EyeInner} {
		if color != "" && !hexColor.MatchString(color) {
			return fmt.Errorf("-%s 必须是 #RRGGBB", name)
		}
	}
	if !strings.Contains("LMQH", o.Level) || len(o.Level) != 1 {
		return errors.New("-level 必须是 L/M/Q/H")
	}
	if o.Version < 1 || o.Version > 40 {
		return errors.New("-qrversion 必须在 1-40")
	}
	if o.Size < 100 || o.Size > 2000 {
		return errors.New("-size 必须在 100-2000")
	}
	if o.Margin < 0 || o.Margin > 20 {
		return errors.New("-margin 必须在 0-20")
	}
	if _, ok := dotTypes[o.Dot]; !ok {
		return fmt.Errorf("未知码点形态 %q", o.Dot)
	}
	if _, ok := eyeTypes[o.Eye]; !ok {
		return fmt.Errorf("未知码眼形状 %q", o.Eye)
	}
	if o.LogoFile != "" && o.LogoURL != "" {
		return errors.New("-logo 与 -logo-url 只能使用一个")
	}
	return nil
}

func buildMHConfig(o options) (mhConfig, error) {
	validated := o
	if validated.LogoURL != "" {
		validated.LogoFile = ""
	}
	if err := validateOptions(validated); err != nil {
		return mhConfig{}, err
	}
	eyeOuter, eyeInner, eyeUseFore := o.EyeOuter, o.EyeInner, 0
	if eyeOuter == "" && eyeInner == "" {
		eyeOuter, eyeInner, eyeUseFore = o.Foreground, o.Foreground, 1
	} else {
		if eyeOuter == "" {
			eyeOuter = o.Foreground
		}
		if eyeInner == "" {
			eyeInner = o.Foreground
		}
	}
	return mhConfig{
		LogoURL: o.LogoURL, LogoShape: "rect", LogoPosition: "0", LogoShadow: 0,
		LogoSize: 90, LogoHeight: "auto", Data: "", Level: o.Level,
		Transparent: 0, Background: "#ffffff", Foreground: strings.ToUpper(o.Foreground),
		BlockPixel: 12, MarginBlock: strconv.Itoa(o.Margin), Size: strconv.Itoa(o.Size),
		Version: strconv.Itoa(o.Version), ForeType: 1, ForeColor2: "",
		BodyType: dotTypes[o.Dot], QRCodeEyes: eyeTypes[o.Eye],
		OuterEyeColor: strings.ToUpper(eyeOuter), InnerEyeColor: strings.ToUpper(eyeInner),
		EyeUseFore: eyeUseFore,
	}, nil
}

func buildSaveForm(o options) url.Values {
	v := url.Values{}
	v.Set("info", o.Text)
	v.Set("content", o.Text)
	v.Set("level", o.Level)
	v.Set("size", strconv.Itoa(o.Size))
	v.Set("margin", strconv.Itoa(o.Margin))
	v.Set("version", strconv.Itoa(o.Version))
	v.Set("codetype", "qr")
	v.Set("type", "text")
	v.Set("is_anonymous", "1")
	v.Set("static_create_from", "10003")
	v.Set("code_small_type", "text")
	v.Set("qrcodeconfig[size]", strconv.Itoa(o.Size))
	v.Set("base64", "")
	v.Set("codeimg", "1")
	v.Set("is_pre_create", "1")
	return v
}

func buildPaintForm(o options, s styleTemplate, mh mhConfig) (url.Values, error) {
	if s.LabelTemplateID == 0 {
		s.LabelTemplateID = 171
	}
	if s.StyleTemplateID == 0 {
		s.StyleTemplateID = 215
	}
	if s.FromCaseType == 0 {
		s.FromCaseType = 1
	}
	if s.CodeType == 0 {
		s.CodeType = 3
	}
	mhJSON, err := json.Marshal(mh)
	if err != nil {
		return nil, err
	}
	themeJSON, err := json.Marshal(map[string]string{"color": strings.ToUpper(o.Background)})
	if err != nil {
		return nil, err
	}
	v := url.Values{}
	v.Set("code_type", strconv.Itoa(s.CodeType))
	v.Set("custom_tpl_param_config", s.CustomTemplateParamConfig)
	v.Set("from_case_type", strconv.Itoa(s.FromCaseType))
	v.Set("label_tplid", strconv.Itoa(s.LabelTemplateID))
	v.Set("mh_str", string(mhJSON))
	v.Set("need_qr_img", "1")
	v.Set("need_qr_version", "1")
	v.Set("need_tpl_tip", "1")
	v.Set("return_file", "base64")
	v.Set("style_tplid", strconv.Itoa(s.StyleTemplateID))
	v.Set("theme_bg", string(themeJSON))
	v.Set("tpl_fields_num", "0")
	v.Set("variable_ass_field", s.VariableAssocField)
	v.Set("web_url", o.Text)
	return v, nil
}

func initSession(client *http.Client) error {
	req, _ := http.NewRequest(http.MethodGet, homeURL, nil)
	req.Header.Set("User-Agent", defaultUA)
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	io.Copy(io.Discard, resp.Body)
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP %s", resp.Status)
	}
	return nil
}

func saveStatic(client *http.Client, o options) (saveResponse, error) {
	var out saveResponse
	if err := postFormJSON(client, saveURL, buildSaveForm(o), &out); err != nil {
		return out, err
	}
	if out.Status != 1 || out.Data.QRID.Int64() == 0 {
		return out, fmt.Errorf("status=%d info=%s", out.Status, out.Info)
	}
	return out, nil
}

func paint(client *http.Client, form url.Values) (paintResponse, error) {
	var out paintResponse
	if err := postFormJSON(client, paintURL, form, &out); err != nil {
		return out, err
	}
	if out.Code != 1 || out.Data.Image == "" {
		return out, fmt.Errorf("code=%d msg=%s", out.Code, out.Msg)
	}
	return out, nil
}

func postFormJSON(client *http.Client, endpoint string, form url.Values, dst any) error {
	req, err := http.NewRequest(http.MethodPost, endpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", defaultUA)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Origin", "https://cli.im")
	req.Header.Set("Referer", defaultRefURL)
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP %s: %s", resp.Status, body)
	}
	if err := json.Unmarshal(body, dst); err != nil {
		return fmt.Errorf("解析响应: %w: %s", err, body)
	}
	return nil
}

func uploadLogo(client *http.Client, path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	var body bytes.Buffer
	w := multipart.NewWriter(&body)
	part, err := w.CreateFormFile("Filedata", filepath.Base(path))
	if err != nil {
		return "", err
	}
	if _, err := io.Copy(part, f); err != nil {
		return "", err
	}
	if err := w.WriteField("blacklist", "1"); err != nil {
		return "", err
	}
	if err := w.Close(); err != nil {
		return "", err
	}
	req, err := http.NewRequest(http.MethodPost, uploadURL, &body)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", defaultUA)
	req.Header.Set("Content-Type", w.FormDataContentType())
	req.Header.Set("Origin", "https://cli.im")
	req.Header.Set("Referer", defaultRefURL)
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	var out uploadResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return "", err
	}
	if resp.StatusCode != http.StatusOK || out.Code != 200 || out.Data.URL == "" {
		return "", fmt.Errorf("HTTP %s code=%d msg=%s", resp.Status, out.Code, out.Msg)
	}
	return out.Data.URL, nil
}

func decodeDataURL(s string) ([]byte, error) {
	i := strings.IndexByte(s, ',')
	if i < 0 {
		return nil, errors.New("响应不是 data URL")
	}
	b, err := base64.StdEncoding.DecodeString(strings.ReplaceAll(s[i+1:], `\/`, "/"))
	if err != nil {
		return nil, fmt.Errorf("解码 PNG: %w", err)
	}
	return b, nil
}

func detect(client *http.Client, imageDataURL string) (string, error) {
	form := url.Values{"image_data": {imageDataURL}, "remove_background": {"0"}}
	var out struct {
		Status  int    `json:"status"`
		Message string `json:"message"`
		Data    struct {
			Content string `json:"qrcode_content"`
		} `json:"data"`
	}
	if err := postFormJSON(client, detectURL, form, &out); err != nil {
		return "", err
	}
	if out.Status != 1 {
		return "", fmt.Errorf("status=%d message=%s", out.Status, out.Message)
	}
	return out.Data.Content, nil
}
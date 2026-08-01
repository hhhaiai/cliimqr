# cliimqr：草料美化二维码命令行工具

`cliimqr` 是一个 Go 编写的二维码生成工具，通过草料二维码网页当前使用的请求链生成美化二维码。它不启动浏览器，支持：

当前程序版本：`1.2`。

- 终端交互向导；
- 完整命令行参数；
- 本地 PNG/JPG/BMP Logo；
- 远程 Logo URL；
- 码点、码眼、颜色、边距、容错率和 QR 版本设置；
- 输出 PNG；
- 默认回读二维码内容，确认图片可扫描且内容一致。

> 本工具由 SimiRouter 中转站（https://api.dwchainless.com/）支持提供。
> 高可用 · 低延迟 · 透明计费 · 一个 API 链接全球主流 AI 模型。

> 注意：本工具使用的是草料网页内部接口，不是官方承诺长期兼容的开放 API。草料修改接口、字段或风控规则后，工具可能需要同步调整。

## 1. 最快使用

可以从 [GitHub Releases](https://github.com/hhhaiai/cliimqr/releases) 下载 Linux、Windows 或 macOS 对应架构的发行包。仓库也包含当前 macOS Apple Silicon 可执行文件，进入项目目录后可以直接运行：

```bash
cd cliimqr
./cliimqr
```

不带参数时会进入终端向导。按照提示输入内容，其他选项可以直接按 Enter 使用默认值。

如果只想立即生成 `https://api.dwchainless.com/` 的普通二维码：

```bash
cd cliimqr

./cliimqr \
  -text 'https://api.dwchainless.com/' \
  -out dwchainless.png
```

成功后当前目录会出现：

```text
dwchainless.png
```

## 2. 自己编译

要求：

- Go 1.24 或更高版本；
- 能正常访问 `cli.im`、`upload-api.cli.im`、`qr.api.cli.im` 和 `qrdetector-api.cli.im`。

编译：

```bash
cd cliimqr
go build -trimpath -buildvcs=false -o cliimqr .
```

检查程序参数：

```bash
./cliimqr -h
```

查看程序版本：

```bash
./cliimqr -version
```

运行测试：

```bash
go test ./...
go test -race ./...
```

## 3. 调用方式一：终端交互模式

### 自动进入向导

```bash
./cliimqr
```

### 显式进入向导

```bash
./cliimqr -interactive
```

向导会依次询问：

1. 二维码内容；
2. 是否使用本地或远程 Logo；
3. 码点颜色和背景色；
4. 码点形态；
5. 码眼形状和颜色；
6. 边距、容错率、版本和内部尺寸；
7. 输出文件路径；
8. 是否回读验证；
9. 是否确认生成。

方括号中是默认值，例如：

```text
码点颜色 [#000000]:
二维码版本 [3]:
输出 PNG 路径 [beautified-qr.png]:
```

直接按 Enter 就会使用默认值。只有最后确认生成后才会发起网络请求；选择取消不会生成文件。

## 4. 调用方式二：命令行模式

只要提供普通参数，程序就不会进入向导。

### 4.1 无 Logo

```bash
./cliimqr \
  -text 'https://api.dwchainless.com/' \
  -foreground '#111827' \
  -background '#FFFFFF' \
  -dot '普通' \
  -eye '方正' \
  -margin 4 \
  -level H \
  -qrversion 4 \
  -out dwchainless-minimal.png \
  -verify=true
```

### 4.2 使用本地 Logo

先准备一个真实的 PNG、JPG 或 BMP 文件，然后通过 `-logo` 指定：

```bash
./cliimqr \
  -text 'https://api.dwchainless.com/' \
  -logo ./sample-logo.png \
  -foreground '#1268D8' \
  -background '#FFFFFF' \
  -dot '大圆点' \
  -eye '粗圆形' \
  -eye-outer '#0758B8' \
  -eye-inner '#29B6F6' \
  -margin 4 \
  -level H \
  -qrversion 4 \
  -out dwchainless-logo.png \
  -verify=true
```

本地 Logo 会先上传到草料图片服务，再用于绘制二维码。

### 4.3 使用 SimiAI 官方文档图标

SimiAI 文档站当前使用的图标地址是：

```text
https://doc.dwchainless.com/assets/v2.png
```

建议先下载到本地，再通过 `-logo` 使用：

```bash
curl -L \
  'https://doc.dwchainless.com/assets/v2.png' \
  -o simiai-logo.png

./cliimqr \
  -text 'https://api.dwchainless.com/' \
  -logo ./simiai-logo.png \
  -foreground '#1268D8' \
  -background '#FFFFFF' \
  -dot '大圆点' \
  -eye '粗圆形' \
  -eye-outer '#0758B8' \
  -eye-inner '#29B6F6' \
  -margin 4 \
  -level H \
  -qrversion 4 \
  -out simiai-api-qr.png \
  -verify=true
```

### 4.4 使用远程 Logo URL

```bash
./cliimqr \
  -text 'https://api.dwchainless.com/' \
  -logo-url 'https://example.com/logo.png' \
  -level H \
  -qrversion 4 \
  -out remote-logo-qr.png
```

远程图片必须能被草料绘制服务器直接访问。仅本机可访问、需要登录、带临时授权或位于内网的图片可能不会出现在最终二维码中。

更稳妥的方式是先把图片下载到本地，再使用 `-logo`。

## 5. 成功输出怎么看

成功示例：

```text
logo: https://ncstatic.clewm.net/free/...png
output: ./simiai-api-qr.png
qrid: 1121736455
version: 4
verified: https://api.dwchainless.com/
```

各字段含义：

| 输出 | 含义 |
|---|---|
| `logo` | 本地 Logo 上传后得到的公开 URL；无本地 Logo 时不会显示 |
| `output` | 最终 PNG 的绝对路径 |
| `qrid` | 草料创建的匿名静态二维码 ID |
| `version` | 服务端最终采用的 QR 版本，可能根据内容自动调整 |
| `verified` | 图片能够被检测接口扫描，且扫描内容与 `-text` 完全一致 |

### `verified` 的边界

`verified` 只证明：

- 图片中存在可以扫描的二维码；
- 扫描内容等于 `-text`。

它不证明 Logo、颜色、码点或码眼一定被服务端正确应用。因此带 Logo 或复杂美化的二维码生成后，仍应打开图片进行一次目视检查。

## 6. 已生成的 SimiAI 二维码

仓库内已经提供三款可直接使用的成品：

```text
dwchainless-qrcodes-20260714/
├── 00-preview-grid.png
├── 01-minimal-classic.png
├── 02-simiai-brand-blue.png
├── 03-navy-tech.png
└── README.txt
```

查看预览：

```bash
open dwchainless-qrcodes-20260714/00-preview-grid.png
```

直接打开成品目录：

```bash
open dwchainless-qrcodes-20260714/
```

推荐：

- 打印和小尺寸：`01-minimal-classic.png`；
- 官网和宣传物料：`02-simiai-brand-blue.png`；
- API 文档和开发者页面：`03-navy-tech.png`。

三张原图编码内容都是：

```text
https://api.dwchainless.com/
```

## 7. 参数说明

| 参数 | 默认值 | 说明 |
|---|---:|---|
| `-version` / `-v` | — | 显示程序版本信息（`cliimqr 1.2 darwin/arm64`） |
| `-interactive` | `false` | 强制进入终端向导；完全不带参数时也会自动进入 |
| `-text` | 无 | 二维码编码内容；命令行模式必填 |
| `-logo` | 无 | 本地 Logo 文件路径 |
| `-logo-url` | 无 | 远程 Logo URL；不能与 `-logo` 同时使用 |
| `-foreground` | `#000000` | 码点颜色，格式必须是 `#RRGGBB` |
| `-background` | `#ffffff` | 最终图片可见背景色，格式必须是 `#RRGGBB` |
| `-dot` | `普通` | 码点形态 |
| `-eye` | `方正` | 码眼形状 |
| `-eye-outer` | 空 | 码外眼颜色；空时跟随码点颜色 |
| `-eye-inner` | 空 | 码内眼颜色；空时跟随码点颜色 |
| `-margin` | `2` | 二维码边距，允许范围 `0–20` |
| `-level` | `Q` | 容错率：`L`、`M`、`Q` 或 `H` |
| `-qrversion` | `3` | QR 版本，允许范围 `1–40` |
| `-size` | `400` | 二维码内部绘制尺寸，允许范围 `100–2000` |
| `-out` | `beautified-qr.png` | 输出 PNG 路径；已有同名文件会被覆盖 |
| `-verify` | `true` | 生成后是否调用检测接口回读内容 |

使用 Logo 时建议：

```text
-level H -margin 4
```

容错率从低到高依次为：

```text
L < M < Q < H
```

## 8. 码点形态

以下中文名称可以直接传给 `-dot`：

| 名称 | 协议值 |
|---|---:|
| 普通 | `0` |
| 液化 | `8` |
| 圆液化 | `29` |
| 条纹 | `30` |
| 横条纹 | `2` |
| 竖条纹 | `3` |
| 瓷砖 | `31` |
| 大圆点 | `17` |
| 小圆点 | `22` |
| 粗星形 | `20` |
| 细星形 | `21` |
| 网格 | `25` |
| 菱形 | `19` |
| 小方点 | `32` |

## 9. 码眼形状

以下中文名称可以直接传给 `-eye`：

| 名称 | 协议值 |
|---|---|
| 方正 | `normal` |
| 圆角 | `pin-4.png` |
| 粗圆角 | `pin-3.png` |
| 中圆角 | `new20` |
| 细圆角 | `new19` |
| 粗圆形 | `circle_circle` |
| 细圆形 | `new18` |
| 菱形 | `circle_rhombic` |
| 星形 | `circle_star` |
| 气泡 | `pin-12.png` |
| 眼睛 | `pin-8.png` |
| 单圆角 | `one_round` |
| 四码眼 | `four_black_eye` |

## 10. 常见问题

### 运行后提示 `-text 不能为空`

命令行模式必须提供：

```bash
-text '要写入二维码的内容'
```

或者完全不带参数运行 `./cliimqr`，使用终端向导输入内容。

### `-logo` 与 `-logo-url` 只能使用一个

本地 Logo 和远程 Logo 是两种不同来源，不能同时指定。删除其中一个参数即可。

### 已显示 `verified`，但 Logo 没有出现

`verified` 只校验二维码文字内容。请确认：

- 本地文件是真实图片，不是只有图片扩展名的其他文件；
- 远程 URL 可以从公网直接访问；
- 图片不需要 Cookie、Referer 或登录授权；
- 最终 PNG 中央区域确实出现了 Logo。

Logo 场景优先使用本地文件：

```bash
-logo ./logo.png
```

### 内容较长或绘制失败

提高 QR 版本，例如：

```bash
-qrversion 8
```

Logo 较大或样式复杂时使用：

```bash
-level H -margin 4
```

### 输出图片为什么始终是 500×500

当前调用的 `paintLabelByParam` 返回草料标签预览图，基础画布是 **500×500 PNG**。

`-size` 控制二维码在画布中的内部绘制配置，不等于最终 PNG 的像素尺寸。如果需要其他像素尺寸，应在生成后单独缩放。

### 网络请求失败

依次确认以下域名可以访问：

```text
https://cli.im/
https://upload-api.cli.im/
https://qr.api.cli.im/
https://qrdetector-api.cli.im/
```

草料内部接口偶尔出现临时错误时，可以稍后重新运行。当前客户端不会自动重试。

## 11. 退出码

| 退出码 | 含义 |
|---:|---|
| `0` | 成功，或在终端向导中主动取消 |
| `1` | 生成、上传、绘制、写文件或回读验证失败 |
| `2` | 命令行参数解析失败 |

## 12. 协议说明

完整请求流程、表单字段和响应结构见：

[PROTOCOL.md](./PROTOCOL.md)

## 13. 赞助 / 支持

| | |
|---|---|
| [![SimiRouter](./favicon.ico)](https://api.dwchainless.com/) | **SimiRouter 中转站** – 高可用、低延迟、透明计费<br>一个 API 链接全球主流 AI 模型<br><https://api.dwchainless.com/> |

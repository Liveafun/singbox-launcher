# Sing-Box Launcher

[![GitHub](https://img.shields.io/badge/GitHub-Leadaxe%2Fsingbox--launcher-blue)](https://github.com/Leadaxe/singbox-launcher)
[![License](https://img.shields.io/badge/License-MIT-green.svg)](LICENSE)
[![Go Version](https://img.shields.io/badge/Go-1.24%2B-blue)](https://golang.org/)

Cross-platform GUI launcher for [sing-box](https://github.com/SagerNet/sing-box) - universal proxy client.

**Repository**: [https://github.com/Leadaxe/singbox-launcher](https://github.com/Leadaxe/singbox-launcher)

**🌐 Languages**: [English](README.md) | [Русский](README_RU.md)

## 🚀 Features

- ✅ **Cross-platform**: Windows, macOS, Linux (Android in development)
- 🎯 **Simple Control**: Start/stop VPN with one button
- 📊 **Clash API Integration**: Manage proxies via Clash-compatible API
- 🔄 **Automatic Configuration Update**: Parse subscriptions and update proxy list
- 📈 **Diagnostics**: IP check, STUN, file verification
- 🔔 **System Tray**: Run from system tray
- 📝 **Logging**: Detailed logs of all operations

## 💡 Why this launcher?

### ❌ The Problem

Most Windows users run sing-box like this:

- 📁 `sing-box.exe` + `config.json` in the same folder  
- ⚫ Black CMD window always open  
- ✏️ To switch a node: edit JSON in Notepad → kill the process → run again  
- 📝 Logs disappear into nowhere  
- 🔄 Manual restart every time you change config

### ✅ The Solution

This launcher solves all of that. Everything is controlled from one clean GUI:

### 🎯 What it gives you

- 🚀 **One-click start/stop for TUN mode**  
- 📝 **Full access to `config.json` inside the launcher**  
  (edit → save → sing-box restarts automatically)
- 🔄 **Auto-parsing of any subscription type**  
  (vless / vmess / trojan / ss / hysteria / tuic)  
  + filters by tags and regex
- 🌐 **Server selection with ping via Clash Meta API**  
- 🔧 **Diagnostics tools**: IP-check, STUN test, process killer  
- 📊 **System tray integration + readable logs**

**🔗 Links:**
- **GitHub:** https://github.com/Leadaxe/singbox-launcher  
- **Example config:** https://github.com/Leadaxe/singbox-launcher/blob/main/bin/config.example.json

## 📋 Requirements

### Windows
- Windows 10/11 (x64)
- [sing-box](https://github.com/SagerNet/sing-box/releases) (executable file)
- [WinTun](https://www.wintun.net/) (wintun.dll) - MIT license, can be distributed

### macOS
- macOS 10.15+ (Catalina or newer)
- [sing-box](https://github.com/SagerNet/sing-box/releases) (executable file)

### Linux
- Modern Linux distribution (x64)
- [sing-box](https://github.com/SagerNet/sing-box/releases) (executable file)

## 📦 Installation

### Windows

1. Download the latest release from [GitHub Releases](https://github.com/Leadaxe/singbox-launcher/releases)
2. Extract the archive to any folder (e.g., `C:\Program Files\singbox-launcher`)
3. Place `config.json` in the `bin\` folder:
   - Copy `config.example.json` to `config.json` and configure it
4. Run `singbox-launcher.exe`
5. **Automatic download** (recommended):
   - Go to the **"Core"** tab
   - Click **"Download"** to download `sing-box.exe` (automatically downloads the correct version for your system)
   - Click **"Download wintun.dll"** if needed (automatically downloads the correct architecture)
   - The launcher will automatically download from GitHub or SourceForge mirror if GitHub is unavailable

### macOS

1. Download the latest release for macOS
2. Extract the archive
3. Place files in the `bin/` folder:
   - `sing-box` - executable file for macOS
   - `config.json` - configuration file

4. Run the application:
   ```bash
   ./singbox-launcher
   ```

### Linux

1. Download the latest release for Linux
2. Extract the archive
3. Place files in the `bin/` folder:
   - `sing-box` - executable file for Linux
   - `config.json` - configuration file

4. Make executable and run:
   ```bash
   chmod +x singbox-launcher
   ./singbox-launcher
   ```

## 📖 Usage

### First Launch

1. Configure `config.json` (see [Configuration](#-configuration) section)
2. **Download sing-box and wintun.dll** (if not already present):
   - Open the **"Core"** tab
   - Click **"Download"** to download `sing-box` (automatically detects your platform)
   - On Windows, click **"Download wintun.dll"** if needed
   - Files will be downloaded to the `bin/` folder automatically
3. Click the **"Start"** button in the **"Core"** tab to start sing-box

### Main Features

#### "Core" Tab
- **Core Status** - Shows sing-box running status (Running/Stopped/Error)
- **Sing-box Ver.** - Displays installed version (clickable on Windows to open file location)
- **Update** button - Download or update sing-box binary
- **WinTun DLL** (Windows only) - Shows wintun.dll status and download button
- Automatic fallback to SourceForge mirror if GitHub is unavailable

#### "Diagnostics" Tab
- **Check Files** - Check for required files
- **Check STUN** - Determine external IP via STUN
- Buttons to check IP on various services

#### "Tools" Tab
- **Open Logs Folder** - Open logs folder
- **Update Config** - Update configuration from subscriptions
- **Open Config Folder** - Open configuration folder
- **Kill Sing-Box** - Force kill sing-box process

#### "Clash API" Tab
- **Test API Connection** - Test Clash API connection
- **Load Proxies** - Load proxy list from selected group
- Switch between proxy servers
- Check latency (ping) for each proxy

### System Tray

The application runs in the system tray. Click the icon to:
- Open the main window
- Start/stop VPN
- Exit the application

## ⚙️ Configuration

### Folder Structure

```
singbox-launcher/
├── bin/
│   ├── sing-box.exe (or sing-box for Unix) - auto-downloaded via Core tab
│   ├── wintun.dll (Windows only) - auto-downloaded via Core tab
│   └── config.json - create from config.example.json
├── logs/
│   ├── singbox-launcher.log
│   ├── sing-box.log
│   └── api.log
└── singbox-launcher.exe (or singbox-launcher for Unix)
```

**Note:** `sing-box` and `wintun.dll` can be downloaded automatically through the **Core** tab. The launcher will:
- Automatically detect your platform (Windows/macOS/Linux) and architecture (amd64/arm64)
- Download the correct version from GitHub or SourceForge mirror (if GitHub is blocked)
- Install files to the correct location

### Configuring config.json

The launcher uses the standard sing-box configuration file. Detailed documentation is available on the [official sing-box website](https://sing-box.sagernet.org/configuration/).

#### Enabling Clash API

To use the "Clash API" tab, add to `config.json`:

```json
{
  "experimental": {
    "clash_api": {
      "external_controller": "127.0.0.1:9090",
      "secret": "your-secret-token-here"
    }
  }
}
```

#### Subscription Parser Configuration

For automatic configuration updates from subscriptions, add at the beginning of `config.json`:

```json
{
  /** @ParcerConfig
  {
    "version": 1,
    "ParserConfig": {
      "proxies": [
        {
          "source": "https://your-subscription-url.com/subscription"
        }
      ],
      "outbounds": [
        {
          "tag": "proxy-out",
          "type": "selector",
          "options": { "interrupt_exist_connections": true },
          "outbounds": {
            "proxies": { "tag": "!/(🇷🇺)/i" },
            "addOutbounds": ["direct-out"],
            "preferredDefault": { "tag": "/🇳🇱/i" }
          },
          "comment": "Proxy group for international connections"
        }
      ]
    }
  }
  */
  // ... rest of configuration
}
```

**📖 For detailed parser configuration documentation, see [ParserConfig.md](ParserConfig.md)**

## 🔄 Subscription Parser - Detailed Logic

The subscription parser is a built-in feature that automatically updates the proxy server list in `config.json` from subscriptions (subscription URLs).

### How It Works

#### 1. Parser Configuration

At the beginning of the `config.json` file, there should be a `/** @ParcerConfig ... */` block with JSON configuration:

```json
{
  /** @ParcerConfig
  {
    "version": 1,
    "ParserConfig": {
      "proxies": [
        {
          "source": "https://your-subscription-url.com/subscription",
          "skip": [ { "tag": "!/🇷🇺/i" } ]
        }
      ],
      "outbounds": [
        {
          "tag": "proxy-out",
          "type": "selector",
          "options": { "interrupt_exist_connections": true },
          "outbounds": {
            "proxies": { "tag": "!/(🇷🇺)/i" },
            "addOutbounds": ["direct-out"],
            "preferredDefault": { "tag": "/🇳🇱/i" }
          },
          "comment": "Proxy group for international connections"
        }
      ]
    }
  }
  */
}
```

#### 2. Update Process

When you click the **"Update Config"** button in the "Tools" tab:

1. **Reading Configuration**
   - Parser finds the `@ParcerConfig` block in `config.json`
   - Extracts subscription URLs from the `proxies[].source` field

2. **Loading Subscriptions**
   - For each URL from `proxies[].source`:
     - Downloads subscription content (Base64 and plain text supported)
     - Decodes and parses the proxy server list

3. **Supported Protocols**
   - ✅ VLESS
   - ✅ VMess
   - ✅ Trojan
   - ✅ Shadowsocks (SS)

4. **Information Extraction**
   - From each URI extracts:
     - **Tag**: left part of comment before `|` (e.g., `🇳🇱Netherlands`)
     - **Comment**: entire text after `#` in URI
     - **Connection parameters**: server, port, UUID, TLS settings, etc.

5. **Node Filtering**

   **`skip` filter** (at subscription level):
   - If a node matches any filter from `skip` - it is skipped
   - Example: `"skip": [ { "tag": "!/🇷🇺/i" } ]` - skip all non-Russian proxies
   
   **`proxies` filter** (at selector level):
   - Determines which nodes will be included in a specific selector
   - Example: `"proxies": { "tag": "!/(🇷🇺)/i" }` - all except Russian

   **Supported filter fields:**
   - `tag` - tag name (case-sensitive, with emoji)
   - `host` - server hostname
   - `label` - original string after `#` in URI
   - `scheme` - protocol (`vless`, `vmess`, `trojan`, `ss`)
   - `fragment` - URI fragment (equals `label`)
   - `comment` - right part of `label` after `|`

   **Pattern formats:**
   - `"literal"` - exact match (case-sensitive)
   - `"!literal"` - negation (does NOT match)
   - `"/regex/i"` - regular expression with `i` flag (case-insensitive)
   - `"!/regex/i"` - negated regular expression

6. **Grouping into Selectors**

   For each object in `outbounds[]`, a selector is created:
   
   - **`tag`**: selector name (used in UI and routing)
   - **`type`**: always `"selector"` for selectors
   - **`outbounds.proxies`**: filter for node selection (OR between objects, AND inside object)
   - **`outbounds.addOutbounds`**: additional tags added to the beginning of the list (e.g., `["direct-out"]`)
   - **`outbounds.preferredDefault`**: filter to determine default proxy
   - **`options`**: additional fields (e.g., `interrupt_exist_connections: true`)
   - **`comment`**: comment displayed before JSON selector

7. **Writing Result**

   Parser finds in `config.json` the block between markers:
   ```
   /** @ParserSTART */
   ... proxies and selectors will be here ...
   /** @ParserEND */
   ```
   
   And replaces it with:
   - List of all filtered proxy servers in JSON format
   - Selectors with proxy grouping according to specified rules
   - Comments from original URIs

### Important Notes

1. **Stop sing-box before updating**
   - Clash API may react to intermediate file
   - Use "Stop VPN" button before "Update Config"

2. **Markers are required**
   - `/** @ParserSTART */` and `/** @ParserEND */` must be in `config.json`
   - Without them, parser doesn't know where to insert the result

3. **Automatic normalization**
   - Incorrect flag `🇪🇳` is automatically replaced with `🇬🇧`
   - Normalization logic can be extended in parser code

4. **UI Integration**
   - "Clash API" tab automatically picks up selector list
   - By default, selector from `route.final` is selected (if matches)
   - Can be switched via dropdown list

5. **Multiple Subscriptions**
   - Multiple subscriptions can be specified in `proxies[]` array
   - All nodes will be merged and filtered together

**📖 For detailed parser configuration, see [ParserConfig.md](ParserConfig.md)**

## 🏗️ Project Architecture

```
singbox-launcher/
├── api/              # Clash API client
├── assets/           # Icons and resources
├── bin/              # Executables and configuration
├── build/            # Build scripts
├── core/             # Core application logic
├── internal/         # Internal packages
│   └── platform/     # Platform-specific code
│       ├── platform_windows.go
│       ├── platform_darwin.go
│       └── platform_common.go
├── ui/               # User interface
├── logs/             # Application logs
├── main.go           # Entry point
├── go.mod            # Go dependencies
└── README.md         # This file
```

### Cross-platform

The project uses build tags for conditional compilation of platform-specific code:

- `//go:build windows` - code for Windows
- `//go:build darwin` - code for macOS
- `//go:build linux` - code for Linux

Platform-specific functions are in the `internal/platform` package.

## 🐛 Troubleshooting

### Sing-box won't start

1. **Download sing-box** if missing:
   - Go to the **"Core"** tab
   - Click **"Download"** to download sing-box automatically
   - On Windows, also download `wintun.dll` if TUN mode is used
2. Check that `sing-box.exe` (or `sing-box`) file exists in the `bin/` folder
2. Check `config.json` correctness
3. Check logs in the `logs/` folder

### Clash API not working

1. Make sure `experimental.clash_api` is enabled in `config.json`
2. Check that sing-box is running
3. Check logs in `logs/api.log`

### Permission issues (Linux/macOS)

On Linux/macOS, administrator rights may be required to create TUN interface:

```bash
sudo ./singbox-launcher
```

Or configure permissions via `setcap`:

```bash
sudo setcap cap_net_admin+ep ./singbox-launcher
```

## 🔨 Building from Source

### Prerequisites

- Go 1.24 or newer
- Git
- For Windows: [rsrc](https://github.com/akavel/rsrc) for embedding icons (optional)

### Windows

**Requirements:**
- Go 1.24 or newer ([download](https://go.dev/dl/))
- **C Compiler (GCC)** - REQUIRED! ([TDM-GCC](https://jmeubank.github.io/tdm-gcc/) or [MinGW-w64](https://www.mingw-w64.org/))
- CGO (enabled by default)
- Optional: `rsrc` for embedding icon (`go install github.com/akavel/rsrc@latest`)

**⚠️ Important:** If you see error `gcc: executable file not found`, install GCC (see [BUILD_WINDOWS.md](BUILD_WINDOWS.md) "Troubleshooting" section)

**Build:**

1. Clone the repository:
```batch
git clone https://github.com/Leadaxe/singbox-launcher.git
cd singbox-launcher
```

2. Run the build script:
```batch
build\build_windows.bat
```

Or manually:
```batch
go mod tidy
go build -buildvcs=false -ldflags="-H windowsgui -s -w" -o singbox-launcher.exe
```

**Detailed instructions:** See [BUILD_WINDOWS.md](BUILD_WINDOWS.md)

### macOS

```bash
# Clone the repository
git clone https://github.com/Leadaxe/singbox-launcher.git
cd singbox-launcher

# Install dependencies
go mod download

# Build the project
chmod +x build/build_darwin.sh
./build/build_darwin.sh
```

Or manually:
```bash
GOOS=darwin GOARCH=amd64 go build -buildvcs=false -ldflags="-s -w" -o singbox-launcher
```

### Linux

```bash
# Clone the repository
git clone https://github.com/Leadaxe/singbox-launcher.git
cd singbox-launcher

# Install dependencies
go mod download

# Build the project
chmod +x build/build_linux.sh
./build/build_linux.sh
```

Or manually:
```bash
GOOS=linux GOARCH=amd64 go build -buildvcs=false -ldflags="-s -w" -o singbox-launcher
```

## 🤝 Contributing

We welcome contributions to the project! Please:

1. Fork the repository
2. Create a branch for your feature (`git checkout -b feature/AmazingFeature`)
3. Commit your changes (`git commit -m 'Add some AmazingFeature'`)
4. Push to the branch (`git push origin feature/AmazingFeature`)
5. Open a Pull Request

### Code Style

- Follow Go standards: `gofmt`, `golint`
- Add comments to public functions
- Write tests for new functionality

## 📄 License

This project is distributed under the MIT license. See the [LICENSE](LICENSE) file for details.

## 🙏 Acknowledgments

- [SagerNet/sing-box](https://github.com/SagerNet/sing-box) - for excellent proxy client
- [Fyne](https://fyne.io/) - for cross-platform UI framework
- All project contributors

## 📞 Support

- **Issues**: [GitHub Issues](https://github.com/Leadaxe/singbox-launcher/issues)
- **Discussions**: [GitHub Discussions](https://github.com/Leadaxe/singbox-launcher/discussions)

## 🔮 Future Plans

- [ ] Automatic application updates
- [ ] Dark theme
- [ ] Multi-language support
- [ ] Traffic statistics graphs
- [ ] Integration with other VPN protocols

---

**Note**: This project is not affiliated with the official sing-box project. This is an independent development for convenient sing-box management.

# Decisions

[日本語](decisions.md)

What was decided before going public, and why (2026-11-24).

| # | Topic | Decision | Why |
|---|---|---|---|
| 1 | License | **MIT or Apache-2.0** (dual) | Easy for companies to adopt; Apache-2.0 covers patents. Same as SolidCraft |
| 2 | Name | **rungkit**, Go module `github.com/Piyoone3939/rungkit` | So `go install` works |
| 3 | Languages | **Japanese and English** | Reach Mitsubishi users in Japan and OSS users abroad |
| 4 | Safety and scope | **Never connects to or writes a real PLC** | See "Safety" |
| 5 | Distribution | **GitHub Releases binaries + `go install` + browser version + one-line MCP setup** | See "Distribution" |
| 6 | Relation to OpenPLC | **Complement, not compete** | See "OpenPLC" |
| 7 | Vendor to deepen next | **Mitsubishi** | FX5 profile (octal X/Y) and GX Works3 import first |
| 8 | GUI | **Browser first** | See "GUI" |
| 9 | Format stability | **May change during 0.x; frozen at v1.0** | See "Compatibility" |
| 10 | Scope | **Focus on Ladder Diagram** | See "Scope" |

## 3. Languages

- Messages are written in Japanese, which serves as the message ID, and translated through an English catalog ([internal/domain/i18n](../internal/domain/i18n/)). A test checks that every Japanese string in the code has an English entry with the same format verbs
- The language comes from `-lang ja|en`, then `RUNGKIT_LANG`, then `LANG` and friends, then the OS setting (the Windows user locale)
- README and docs exist in both languages (`*.md` and `*.en.md`; the English README is `README.md`, the Japanese one `README.ja.md`)
- MCP instructions and tool descriptions are in English: agents read them, whatever the user's language

## 4. Safety

- rungkit **does not talk to real PLCs** (no download, monitoring or forcing). Engineers review its output and load it with the vendor's tool (GX Works3 and so on)
- Ladder written by an AI is checked with `check` and scenarios (`.ldtest`) before use. Safety functions such as emergency stops still belong in wiring and certified safety devices, not in software
- If online features are ever added, they start read-only (monitoring); writing is a separate decision
- The MCP server never reads or writes files outside its working folder (`rungkit mcp -root DIR`)
- Mitsubishi Electric, MELSEC, GX Works and others are trademarks of their owners. rungkit is not affiliated with them

## 5. Distribution

SolidCraft shipped binaries for every OS on GitHub Releases, a version that runs in the browser and a one-line MCP setup for Claude Code, and spread through Show HN and tech media. rungkit follows the same pattern:

1. **GitHub Releases** with Windows, macOS and Linux binaries (GoReleaser, built automatically on a tag)
2. **`go install github.com/Piyoone3939/rungkit/cmd/rungkit@latest`**
3. **A browser version** on GitHub Pages; trying without installing spreads best
4. **One-line MCP setup** (`claude mcp add rungkit -- rungkit mcp`) at the top of the README, and a listing in MCP registries at release
5. Announce on Show HN (English) and Zenn / Qiita (Japanese)

## 6. OpenPLC

OpenPLC Editor is adding AI features, but tied to its own assistant, without MCP or Mitsubishi support. rungkit **does not replace editors; it is the layer where an AI writes ladder and checks its own work, with Japanese vendor support**. Through PLCopen XML it works with Beremiz, OpenPLC and CODESYS, so users keep their editor.

## 8. GUI

**Browser first** (Go compiled to WebAssembly, served from GitHub Pages):

- Nothing to install. Factory and school PCs often lack admin rights, so this matters a lot
- Trying SolidCraft in the browser was part of why it spread
- The same UI can be wrapped as a desktop app later (Wails or similar) if needed

## 9. Compatibility

- During 0.x, `.ldt`, JSON, `.ldtest` and the MCP tools may change. Changes go in the CHANGELOG and `rungkit fmt -w` migrates files
- v1.0 freezes the formats; later versions keep reading old files

## 10. Scope

| | |
|---|---|
| Yes | Ladder Diagram, Mitsubishi iQ-R / FX5, IEC, PLCopen XML, simulator, MCP, browser UI |
| Later | Function blocks placed inside a rung (IEC style, after the UI), Keyence, Omron |
| No | Editing SFC or ST (ST is export only), talking to real PLCs |

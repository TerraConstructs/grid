# Demo Directory

This directory contains VHS (Video Handling System) demo scripts for creating animated demonstrations of the `gridapi` & `gridctl`.

## Prerequisites

Install VHS and dependencies by Charm:
```bash
# Using Go
go install github.com/charmbracelet/vhs@latest

# Using Homebrew (macOS)
brew install vhs
# also install dependencies ffmpeg, ttyd

# Using package managers (Linux)
# See: https://github.com/charmbracelet/vhs#installation
```

## Recording Demos

### Generate the Demo GIF
```bash
# From the project root
vhs demo/demo.tape
```

This will create `examples/demo/demo.gif` showing the complete grid examples/terraform demo.

### Customizing the Demo

Edit `demo.tape` to:
- Change terminal dimensions (`Set Width/Height`)
- Adjust timing (`Sleep` durations)
- Modify font size (`Set FontSize`)
- Add/remove workflow steps

## Integration

The generated `demo.gif` is designed to be embedded in:
- Main project README
- Documentation sites
- Social media demonstrations
- Project presentations

## Reference

For more VHS documentation and examples, see:
- [VHS Repository](https://github.com/charmbracelet/vhs)
- [Building Bubble Tea Programs Blog](https://leg100.github.io/en/posts/building-bubbletea-programs/#10-record-demos-and-screenshots-on-vhs)

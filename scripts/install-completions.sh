#!/usr/bin/env bash
#
# Installs shell completion for doctordock and the ddock alias.
#
#   make completions
#
# Cobra generates the completion script; this only works out where the current
# shell looks for it, which differs on every platform and package manager.
#
set -uo pipefail

BINARY="${1:-doctordock}"

if ! command -v "$BINARY" >/dev/null 2>&1 && [ ! -x "$BINARY" ]; then
  echo "cannot find the doctordock binary: $BINARY" >&2
  exit 1
fi

# The completion script embeds the command name it completes, so the alias
# needs its own copy generated under that name rather than a symlink.
generate() {
  local shell="$1" name="$2" dest="$3"
  "$BINARY" completion "$shell" 2>/dev/null | sed "s/doctordock/$name/g" > "$dest"
}

current_shell=$(basename "${SHELL:-bash}")

case "$current_shell" in
  zsh)
    # Homebrew's site-functions is on fpath for anyone who runs `brew shellenv`,
    # which its own installer adds to the profile.
    for dir in /opt/homebrew/share/zsh/site-functions /usr/local/share/zsh/site-functions "$HOME/.zfunc"; do
      if [ -w "$dir" ] 2>/dev/null; then target="$dir"; break; fi
    done
    if [ -z "${target:-}" ]; then
      target="$HOME/.zfunc"
      mkdir -p "$target"
      echo "note: add this to ~/.zshrc before compinit:"
      echo "    fpath=($target \$fpath)"
    fi

    "$BINARY" completion zsh > "$target/_doctordock"
    generate zsh ddock "$target/_ddock"
    echo "installed zsh completion into $target"
    # compinit caches its dump, so a newly added completion is invisible until
    # the cache is rebuilt.
    echo "run this once to pick it up:  rm -f ~/.zcompdump* && exec zsh"
    ;;

  bash)
    for dir in /opt/homebrew/etc/bash_completion.d /usr/local/etc/bash_completion.d /etc/bash_completion.d "$HOME/.local/share/bash-completion/completions"; do
      if [ -w "$dir" ] 2>/dev/null; then target="$dir"; break; fi
    done
    if [ -z "${target:-}" ]; then
      target="$HOME/.local/share/bash-completion/completions"
      mkdir -p "$target"
    fi

    "$BINARY" completion bash > "$target/doctordock"
    generate bash ddock "$target/ddock"
    echo "installed bash completion into $target"
    echo "start a new shell to pick it up"
    ;;

  fish)
    target="$HOME/.config/fish/completions"
    mkdir -p "$target"
    "$BINARY" completion fish > "$target/doctordock.fish"
    generate fish ddock "$target/ddock.fish"
    echo "installed fish completion into $target"
    ;;

  *)
    echo "no completion installer for '$current_shell'."
    echo "generate one manually: $BINARY completion --help"
    exit 1
    ;;
esac

#!/usr/bin/env bash

#Usage ex : ./scripts/gen_formula.sh 1.1.0
VERSION=$1
TAR_URL="https://github.com/YOUR_USERNAME/zx/releases/download/v$VERSION/zx-macos.tar.gz"
SHA=$(shasum -a 256 release/zx-macos.tar.gz | awk '{print $$1}')

cat <<EOF > zx.rb
class Zx < Formula
  desc "Fast, colorized grep-like CLI with fuzzy suggestions"
  homepage "https://github.com/YOUR_USERNAME/zx"
  url "$TAR_URL"
  sha256 "$SHA"
  license "MIT"

  def install
    bin.install "zx-macos" => "zx"
  end

  test do
    system "#{bin}/zx", "--help"
  end
end
EOF

echo "✅ Homebrew formula written to zx.rb"

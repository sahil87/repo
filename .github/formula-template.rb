class Repo < Formula
  desc "Locate, open, list, and clone repos from repos.yaml"
  homepage "https://github.com/sahil87/repo"
  version "VERSION_PLACEHOLDER"
  license "MIT"

  on_macos do
    on_arm do
      url "https://github.com/sahil87/repo/releases/download/v#{version}/repo-darwin-arm64.tar.gz"
      sha256 "SHA_DARWIN_ARM64"
    end
    on_intel do
      url "https://github.com/sahil87/repo/releases/download/v#{version}/repo-darwin-amd64.tar.gz"
      sha256 "SHA_DARWIN_AMD64"
    end
  end

  on_linux do
    on_arm do
      url "https://github.com/sahil87/repo/releases/download/v#{version}/repo-linux-arm64.tar.gz"
      sha256 "SHA_LINUX_ARM64"
    end
    on_intel do
      url "https://github.com/sahil87/repo/releases/download/v#{version}/repo-linux-amd64.tar.gz"
      sha256 "SHA_LINUX_AMD64"
    end
  end

  def install
    bin.install "repo"
  end

  test do
    assert_match version.to_s, shell_output("#{bin}/repo --version")
  end
end

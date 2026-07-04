# typed: false
# frozen_string_literal: true

class GhRepoDashboard < Formula
  desc "K9s-inspired Bubble Tea TUI for managing multiple git and jj repositories"
  homepage "https://github.com/kyleking/gh-repo-dashboard"
  license "MIT"
  version "0.1.0"

  on_macos do
    if Hardware::CPU.arm?
      url "#{homepage}/releases/download/v#{version}/gh-repo-dashboard-darwin-arm64"
      sha256 "REPLACE_WITH_SHA256_FOR_DARWIN_ARM64"
    else
      url "#{homepage}/releases/download/v#{version}/gh-repo-dashboard-darwin-amd64"
      sha256 "REPLACE_WITH_SHA256_FOR_DARWIN_AMD64"
    end
  end

  on_linux do
    if Hardware::CPU.arm?
      url "#{homepage}/releases/download/v#{version}/gh-repo-dashboard-linux-arm64"
      sha256 "REPLACE_WITH_SHA256_FOR_LINUX_ARM64"
    else
      url "#{homepage}/releases/download/v#{version}/gh-repo-dashboard-linux-amd64"
      sha256 "REPLACE_WITH_SHA256_FOR_LINUX_AMD64"
    end
  end

  def install
    binary_name = "gh-repo-dashboard-#{OS.kernel_name.downcase}-#{Hardware::CPU.arch}"
    bin.install binary_name => "gh-repo-dashboard"
  end

  test do
    assert_match version.to_s, shell_output("#{bin}/gh-repo-dashboard --version")
  end
end

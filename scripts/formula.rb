#!/usr/bin/env ruby
require "digest"

version, repository = ARGV
abort "usage: scripts/formula.rb VERSION [OWNER/REPOSITORY]" unless version&.match?(/\Av\d[\w.\-]*\z/)
name = "themeset"
platforms = %w[darwin-arm64 darwin-amd64 linux-arm64 linux-amd64]
checksums = platforms.to_h do |platform|
  archive = "#{name}-#{version.delete_prefix('v')}-#{platform}.tar.gz"
  [platform, [archive, Digest::SHA256.file(File.join("dist", archive)).hexdigest]]
end
File.write("dist/checksums.txt", checksums.values.map { |file, sha| "#{sha}  #{file}\n" }.join)
if repository.nil? || repository.empty?
  warn "Archives and checksums created. Pass OWNER/REPOSITORY to generate a release formula."
  exit
end
abort "repository must be OWNER/REPOSITORY" unless repository.match?(%r{\A[\w.-]+/[\w.-]+\z})
base = "https://github.com/#{repository}/releases/download/#{version}"
sections = {"macos" => "darwin", "linux" => "linux"}.map do |os, platform|
  arm, x86 = checksums.fetch("#{platform}-arm64"), checksums.fetch("#{platform}-amd64")
  <<~RUBY
    on_#{os} do
      if Hardware::CPU.arm?
        url "#{base}/#{arm[0]}"
        sha256 "#{arm[1]}"
      else
        url "#{base}/#{x86[0]}"
        sha256 "#{x86[1]}"
      end
    end
  RUBY
end
formula = <<~RUBY
  class Themeset < Formula
    desc "Apply existing app themes together with explicit custom fallbacks"
    homepage "https://github.com/#{repository}"
    version "#{version.delete_prefix('v')}"
    license "MIT"

  #{sections.map { |s| s.lines.map { |line| "  #{line}" }.join }.join("\n")}
    def install
      bin.install "themeset"
      pkgshare.install "licenses", "THIRD_PARTY_NOTICES.md"
      generate_completions_from_executable(bin/"themeset", "completion", shells: [:bash, :zsh, :fish])
    end

    test do
      assert_match "#{version}", shell_output("\#{bin}/themeset --version")
      assert_match "ghostty", shell_output("\#{bin}/themeset tools")
    end
  end
RUBY
File.write("dist/themeset.rb", formula)

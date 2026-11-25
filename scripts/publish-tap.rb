#!/usr/bin/env ruby
require "base64"
require "json"
require "open3"

repository, formula = ARGV
abort "usage: scripts/publish-tap.rb OWNER/TAP FORMULA.rb" unless repository&.match?(%r{\A[\w.-]+/[\w.-]+\z}) && formula && File.file?(formula)
abort "GH_TOKEN or GITHUB_TOKEN is required" if ENV.fetch("GH_TOKEN", ENV.fetch("GITHUB_TOKEN", "")).empty?
name = File.basename(formula)
abort "expected a Ruby formula filename" unless name.match?(/\A[a-z][a-z0-9-]*\.rb\z/)
endpoint = "repos/#{repository}/contents/Formula/#{name}"
body, error, status = Open3.capture3("gh", "api", endpoint)
sha = nil
if status.success?
  sha = JSON.parse(body).fetch("sha")
elsif !error.include?("HTTP 404")
  abort "Cannot inspect Homebrew tap: #{error}"
end
payload = {
  message: "chore: update #{name.delete_suffix('.rb')} formula",
  content: Base64.strict_encode64(File.binread(formula))
}
payload[:sha] = sha if sha
_, error, status = Open3.capture3("gh", "api", "--method", "PUT", endpoint, "--input", "-", stdin_data: JSON.generate(payload))
abort "Cannot update Homebrew tap: #{error}" unless status.success?
puts "Updated #{repository}/Formula/#{name}"

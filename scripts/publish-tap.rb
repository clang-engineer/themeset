#!/usr/bin/env ruby
require "fileutils"
require "open3"
require "tmpdir"

repository, formula = ARGV
abort "usage: scripts/publish-tap.rb OWNER/TAP FORMULA.rb" unless repository&.match?(%r{\A[\w.-]+/[\w.-]+\z}) && formula && File.file?(formula)
name = File.basename(formula)
abort "expected a Ruby formula filename" unless name.match?(/\A[a-z][a-z0-9-]*\.rb\z/)
formula = File.expand_path(formula)

def run(*arguments)
  abort "Command failed: #{arguments.join(' ')}" unless system(*arguments)
end

Dir.mktmpdir("themeset-tap-") do |directory|
  run("git", "clone", "--quiet", "git@github.com:#{repository}.git", directory)
  run("git", "-C", directory, "checkout", "-B", "main")
  destination = File.join(directory, "Formula", name)
  FileUtils.mkdir_p(File.dirname(destination))
  FileUtils.cp(formula, destination)
  run("git", "-C", directory, "add", "Formula/#{name}")
  _, error, status = Open3.capture3("git", "-C", directory, "diff", "--cached", "--quiet")
  if status.success?
    puts "#{repository}/Formula/#{name} is already current"
  else
    abort error unless status.exitstatus == 1
    run("git", "-C", directory, "-c", "user.name=github-actions[bot]",
      "-c", "user.email=41898282+github-actions[bot]@users.noreply.github.com",
      "commit", "-m", "chore: update #{name.delete_suffix('.rb')} formula")
    run("git", "-C", directory, "push", "origin", "HEAD:main")
    puts "Updated #{repository}/Formula/#{name}"
  end
end

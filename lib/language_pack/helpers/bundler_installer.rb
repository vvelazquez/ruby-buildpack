require_relative '../../../compile-extensions/lib/dependencies'
require 'yaml'

class LanguagePack::BundlerInstaller
  def initialize(dep_dir, stack)
    @dep_dir = dep_dir
  end

  def install
    Dir.chdir(@dep_dir) do
      FileUtils.mkdir_p(binary_path)
      Dir.chdir(binary_path) do
        fetcher.fetch_untar("#{binary_path}.tgz", "--strip-components 1")
      end
    end

    FileUtils.mkdir_p("#{@dep_dir}/bin")
    FileUtils.ln_s("../#{binary_path}/bin/bundle", "#{@dep_dir}/bin/bundle", :force => true)
    FileUtils.ln_s("../#{binary_path}/bin/bundler", "#{@dep_dir}/bin/bundler", :force => true)


    ## TODO refactor
    FileUtils.mkdir_p("#{@dep_dir}/env")
    File.write("#{@dep_dir}/env/GEM_PATH", "#{@dep_dir}/bundler-#{version}:$GEM_PATH")
  end

  def binary_path
    @binary_path || "bundler-#{version}"
  end

  private

  def fetcher
    bundler_base_url = "https://buildpacks.cloudfoundry.org/dependencies/bundler/bundler-#{version}.tgz"
    @fetcher ||= LanguagePack::Fetcher.new(bundler_base_url)
  end

  def version
    return @version if @version
    manifest_path = File.expand_path(File.join(File.dirname(__FILE__), "..", "..", "..", "manifest.yml"))
    dependencies = CompileExtensions::Dependencies.new(YAML.load_file(manifest_path))
    @version = dependencies.newest_patch_version({'name'=>'bundler'})
  end
end

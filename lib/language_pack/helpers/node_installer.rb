class LanguagePack::NodeInstaller
  MODERN_NODE_VERSION = "TO_BE_REPLACED_BY_CF_DEFAULTS"
  MODERN_BINARY_PATH  = "node-v#{MODERN_NODE_VERSION}-linux-x64"
  NODEJS_BASE_URL     = "https://s3pository.heroku.com/node/v#{MODERN_NODE_VERSION}/"

  def initialize(stack)
    @fetcher = LanguagePack::Fetcher.new(NODEJS_BASE_URL)
  end

  def version
    MODERN_NODE_VERSION
  end

  def binary_path
    MODERN_BINARY_PATH
  end

  def install
    puts `pwd`
    FileUtils.mkdir_p("../vendor/node")
    Dir.chdir("../vendor/node") do
      @fetcher.fetch_untar("#{MODERN_BINARY_PATH}.tar.gz")
    end
    `ln -s ../vendor/node/bin/node`
    `ln -s ../vendor/node/bin/npm`
    puts `ls -la`
    # puts `./node -v`
    # puts `./npm -v`
  end

  def self.default_node_version
    bin_path = File.expand_path(File.join(File.dirname(__FILE__), "..", "..", "..", "compile-extensions", "bin"))
    manifest_path = File.expand_path(File.join(File.dirname(__FILE__), "..", "..", "..", "manifest.yml"))
    `#{bin_path}/default_version_for #{manifest_path} node`.chomp
  end
end

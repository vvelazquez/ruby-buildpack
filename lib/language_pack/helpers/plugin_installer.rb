require "language_pack/shell_helpers"

module LanguagePack
  module Helpers
    # Takes an array of plugin names and vendor_url
    # fetches plugins from url, installs them
    class PluginsInstaller
      attr_accessor :plugins, :vendor_url, :buildpack_dir
      include LanguagePack::ShellHelpers

      def initialize(plugins, vendor_url = LanguagePack::Base::VENDOR_URL)
        @plugins    = plugins || []
        @vendor_url = vendor_url
        @buildpack_dir = File.expand_path(File.join(File.dirname(__FILE__), '..', '..', '..'))
      end

      # vendors all the plugins into the slug
      def install
        return true unless plugins.any?
        plugins.each { |plugin| vendor(plugin) }
      end

      def plugin_dir(name = "")
        Pathname.new("vendor/plugins").join(name)
      end

      # vendors an individual plugin
      # @param [String] name of the plugin
      def vendor(name)
        directory = plugin_dir(name)
        return true if directory.exist?

        directory.mkpath
        source_dir = File.join(buildpack_dir, 'plugins', name)
        system "rsync", "-a", source_dir, directory.to_s
      end
    end
  end
end

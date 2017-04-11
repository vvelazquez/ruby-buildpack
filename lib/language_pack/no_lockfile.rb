require "language_pack"
require "language_pack/base"

class LanguagePack::NoLockfile < LanguagePack::Base
  def self.use?
    !File.exists?("Gemfile.lock")
  end

  def name
    "Ruby/NoLockfile"
  end

  def supply
    ## TODO ; we should probably give defaults??
    error "gemfile.lock required. please check it in."
  end

  def finalize
    error "gemfile.lock required. please check it in."
  end
end

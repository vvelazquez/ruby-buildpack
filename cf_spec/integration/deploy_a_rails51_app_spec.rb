require 'cf_spec_helper'

describe 'Rails 5.1 App' do
  subject(:app) do
    Machete.deploy_app(app_name)
  end
  let(:browser) { Machete::Browser.new(app) }

  after do
    Machete::CF::DeleteApp.new.execute(app)
  end

  context 'in an offline environment', :cached do
    let(:app_name) { 'rails51' }

    specify do
      expect(app).to be_running

      browser.visit_path('/')
      expect(browser).to have_body('Hello World')

      expect(app).not_to have_internet_traffic
      expect(app).to have_logged /Downloaded \[file:\/\/.*\]/
    end

  end

  context 'in an online environment', :uncached do
    let(:app_name) { 'rails51' }

    specify do
      expect(app).to be_running

      browser.visit_path('/')
      expect(browser).to have_body('Hello World')
      expect(app).to have_logged /Downloaded \[https:\/\/.*\]/
    end
  end
end

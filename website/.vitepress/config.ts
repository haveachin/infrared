import { defineConfig} from 'vitepress'

// https://vitepress.dev/reference/site-config
export default defineConfig({
  lang: 'en-US',
  title: 'Infrared',
  titleTemplate: ':title | Minecraft Proxy',
  description: 'Minecraft Proxy',
  cleanUrls: true,
  head: [
    [
      'link',
      {
        rel: 'icon',
        type: 'image/x-icon',
        href: '/assets/logo.svg',
      },
    ],
  ],
  themeConfig: {
    // https://vitepress.dev/reference/default-theme-config
    logo: '/assets/logo.svg',

    nav: [
      { text: 'Download', link: '/download' },
      {
        text: 'Guides',
        items: [
          { text: 'Add More Servers', link: '/guide/add-more-servers' },
          { text: 'Add More Gateways', link: '/guide/add-more-gateways' },
          { text: 'Proxy Protocol', link: '/guide/proxy-protocol' },
          { text: 'RealIP', link: '/guide/real-ip' },
        ]
      },
      {
        text: 'Config',
        items: [
          { text: 'Java', link: '/config/java' },
          { text: 'Bedrock', link: '/config/bedrock' },
        ]
      },
      { text: 'OpenAPI Docs', link: 'pathname:///api' },
      { text: 'Donate', link: 'https://ko-fi.com/haveachin' },
    ],

    sidebar: [
      { text: 'What is Infrared?', link: '/what-is-infrared' },
      { text: 'Use Cases', link: '/use-cases' },
      { text: 'Getting Started', link: '/getting-started' },
      {
        text: 'Config',
        items: [
          { text: 'Providers', link: '/config/providers' },
          { text: 'Java', link: '/config/java' },
          { text: 'Bedrock', link: '/config/bedrock' },
          { text: 'Defaults', link: '/config/defaults' },
          { text: 'Docker Labels', link: '/config/docker-labels' },
        ],
      },
      {
        text: 'Plugins',
        items: [
          { text: 'API', link: '/plugin/api' },
          { text: 'Prometheus', link: '/plugin/prometheus' },
          { text: 'Session Validator', link: '/plugin/session-validator' },
          { text: 'Webhooks', link: '/plugin/webhooks' },
        ],
      },
      {
        text: 'Guides',
        items: [
          { text: 'Add More Servers', link: '/guide/add-more-servers' },
          { text: 'Add More Gateways', link: '/guide/add-more-gateways' },
          { text: 'Proxy Protocol', link: '/guide/proxy-protocol' },
          { text: 'RealIP', link: '/guide/real-ip' },
        ]
      },
      { text: 'Troubleshooting', link: '/troubleshooting' },
      { text: 'Open API Docs', link: 'pathname:///api' },
      { text: 'Branding', link: '/branding' },
    ],

    socialLinks: [
      { icon: 'github', link: 'https://github.com/haveachin/infrared' },
      { icon: 'discord', link: 'https://discord.gg/r98YPRsZAx' },
    ],

    footer: {
      message: 'Released under the <a href="https://www.gnu.org/licenses/agpl-3.0.en.html">AGPL-3.0</a>.',
      copyright: 'Copyright © 2019-present Haveachin and Contributors',
    },

    editLink: {
      pattern: 'https://github.com/haveachin/infrared/edit/master/website/:path'
    },
    
    search: {
      provider: 'local'
    },
  }
})

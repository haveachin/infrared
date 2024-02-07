import { defineConfig} from 'vitepress'

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
        href: '/img/logo.svg',
      },
    ],
  ],
  themeConfig: {
    logo: '/img/logo.svg',

    nav: [
      {
        text: 'Features',
        items: [
          { text: 'PROXY Protocol', link: '/features/proxy-protocol' },
          { text: 'Rate Limiter', link: '/features/rate-limiter' },
        ]
      },
      {
        text: 'Config',
        items: [
          { text: 'Global', link: '/config/' },
          { text: 'Proxies', link: '/config/proxies' },
          { text: 'CLI & Env Vars', link: '/config/cli-and-env-vars' },
        ]
      },
      {
        text: 'Donate',
        items: [
          { text: 'PayPal', link: 'https://paypal.me/hendrikschlehlein' },
          { text: 'Ko-Fi', link: 'https://ko-fi.com/haveachin' },
          { text: 'Liberapay', link: 'https://liberapay.com/haveachin' },
        ]
      },
    ],

    sidebar: [
      { text: 'Getting Started', link: '/getting-started' },
      { text: 'Community Projects', link: '/community-projects' },
      {
        text: 'Config',
        items: [
          { text: 'Global', link: '/config/' },
          { text: 'Proxies', link: '/config/proxies' },
          { text: 'CLI & Env Vars', link: '/config/cli-and-env-vars' },
        ],
      },
      {
        text: 'Features',
        items: [
          { text: 'PROXY Protocol', link: '/features/proxy-protocol' },
          {
            text: 'Filters',
            link: '/features/filters',
            items: [
              { text: 'Rate Limiter', link: '/features/rate-limiter' },
            ]
          }
        ]
      },
      { text: 'Report an Issue', link: 'https://github.com/haveachin/infrared/issues' },
      { text: 'Ask in Discussions', link: 'https://github.com/haveachin/infrared/discussions' },
      { text: 'Join our Discord', link: 'https://discord.gg/r98YPRsZAx' },
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
      pattern: 'https://github.com/haveachin/infrared/edit/main/docs/:path'
    },
    
    search: {
      provider: 'local'
    },
  }
})
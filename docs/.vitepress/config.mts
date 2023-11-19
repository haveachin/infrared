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
      {
        text: 'Guides',
        items: [
          { text: 'Proxy Protocol', link: '/guide/proxy-protocol' },
        ]
      },
      {
        text: 'Config',
        items: [
          { text: 'Global', link: '/config/' },
          { text: 'Proxy', link: '/config/proxy' },
        ]
      },
      {
        text: 'Donate',
        items: [
          { text: 'PayPal', link: 'https://paypal.me/hendrikschlehlein' },
          { text: 'Ko-Fi', link: 'https://ko-fi.com/haveachin' },
        ]
      },
    ],

    sidebar: [
      { text: 'Getting Started', link: '/getting-started' },
      {
        text: 'Config',
        items: [
          { text: 'Global', link: '/config/' },
          { text: 'Proxy', link: '/config/proxy' },
        ],
      },
      {
        text: 'Guides',
        items: [
          { text: 'Proxy Protocol', link: '/guide/proxy-protocol' },
        ]
      },
      { text: 'Report an Issue', link: 'https://github.com/haveachin/infrared/issues' },
      { text: 'Discussions', link: 'https://github.com/haveachin/infrared/discussions' },
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
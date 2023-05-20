import { defineConfig} from 'vitepress'

// https://vitepress.dev/reference/site-config
export default defineConfig({
  lang: 'en-US',
  title: 'Infrared',
  titleTemplate: ':title | Minecraft Proxy',
  description: "Minecraft Proxy",
  cleanUrls: true,
  head: [
    [
      'link',
      {
        rel: 'icon',
        type: 'image/x-icon',
        href: 'logo.svg',
      },
    ],
  ],
  themeConfig: {
    // https://vitepress.dev/reference/default-theme-config
    logo: "/logo.svg",

    nav: [
      { text: 'Home', link: '/' },
      {
        text: 'Guides',
        items: [
          { text: 'Item A', link: '/item-1' },
          { text: 'Item B', link: '/item-2' },
          { text: 'Item C', link: '/item-3' }
        ]
      }
    ],

    sidebar: [
      {
        text: 'Examples',
        items: [
          { text: 'Markdown Examples', link: '/markdown-examples' },
          { text: 'Runtime API Examples', link: '/api-examples' }
        ]
      }
    ],

    socialLinks: [
      { icon: 'github', link: 'https://github.com/haveachin/infrared' },
      { icon: 'discord', link: 'https://discord.gg/r98YPRsZAx' },
    ],

    footer: {
      message: 'Released under the AGPL-3.0.',
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

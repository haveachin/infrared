import { defineConfig} from 'vitepress'

// https://vitepress.dev/reference/site-config
export default defineConfig({
  lang: 'en-US',
  title: 'Infrared',
  titleTemplate: ':title | Reverse Proxy',
  description: "A Minecraft Reverse Proxy",
  cleanUrls: true,
  themeConfig: {
    // https://vitepress.dev/reference/default-theme-config
    logo: "/logo.svg",

    nav: [
      { text: 'Home', link: '/' },
      { text: 'Examples', link: '/markdown-examples' }
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
    }
  }
})

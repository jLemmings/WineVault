export default defineNuxtConfig({
  compatibilityDate: '2025-05-15',
  devtools: { enabled: false },
  runtimeConfig: { public: { repositoryUrl: 'https://github.com/jLemmings/WineVault' } },
  css: ['~/assets/main.css', '~/assets/editors.css', '~/assets/scanner.css', '~/assets/viewport.css'],
  nitro: { devProxy: { '/api': { target: 'http://127.0.0.1:8080/api', changeOrigin: true } } },
  routeRules: { '/api/**': { proxy: 'http://127.0.0.1:8080/api/**' } },
  app: { head: { title: 'WineVault — A place for every bottle', meta: [{ name: 'description', content: 'Your personal wine cellar, beautifully organized.' }] } }
})

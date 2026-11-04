# Public website SEO

The build generates public HTML from the actual React pages for `/`, `/investment`,
`/branches`, `/contact` and `/guide/customer`. Thai is the default. English uses the
same paths with `?lang=en`. Public navigation and language switching preserve these
URLs. Other query parameters (for example an album or campaign) are excluded from
the canonical URL. Metadata and sitemap URLs use `https://www.rbcevstation.com`.

## Build and verify

From `apps/web`:

```sh
npm run build
npm run test:seo
npm test
npm run lint
npx playwright test --config playwright.seo.config.ts
```

Deploy the complete `dist` directory with `apps/web/nginx.conf`. Running only
`vite build` skips prerendering and is not a supported production build. The
existing Dockerfile runs `npm`'s equivalent `pnpm build`, which includes the new
prerender step. No Node server is needed at runtime. Vite preview mirrors the
public/private HTML policy for local checks. Vite development updates metadata
but prerendered body content is verified against the production build.

## Indexing and data boundaries

- The generated sitemap includes only the ten public language URLs. It does not
  contain login, account, site IDs, reports, LIFF or administrative URLs.
- Public HTML contains self-referencing canonical URLs, reciprocal `hreflang`
  links, Open Graph/Twitter tags and JSON-LD Organization/WebSite/WebPage data.
  Subpages also contain breadcrumbs. No reviews, return guarantees, operating
  hours, prices or a completed Pracha Uthit project are invented for markup.
- Private pages use `noindex, nofollow` in both the initial HTML and Nginx response
  header. Their URLs are deliberately not disallowed in robots.txt, so crawlers
  can read the noindex rule. API routes and internal prerender storage are excluded
  from crawling. Unknown URLs return HTTP 404, not a successful home page.
- `noindex` is not authorization. Existing server-side authentication and record
  permissions remain responsible for protecting customer data.
- Public HTML is generated without a session, customer records, API calls or
  browser storage. All visitors and crawlers receive the same public content.
- `src/seo/metadata.ts` owns public page metadata. Public page components own
  visible copy. Regenerate the build after changing either.

## Release checks and external follow-up

1. Deploy the tested build using the normal release process. This implementation
   does not deploy, purge Cloudflare caches or modify the production database.
2. On the real domain, check all public language URLs, sitemap/robots content,
   redirects, private noindex headers and genuine 404 responses. Check apex vs
   `www` redirection and HTTPS at the existing Cloudflare edge.
3. Verify ownership of the real domain in Google Search Console, submit
   `https://www.rbcevstation.com/sitemap.xml`, and use URL Inspection on public
   pages. Ownership verification requires the appropriate account/DNS access.
4. Review indexing, queries, clicks and mobile Core Web Vitals using actual field
   data. Rank, traffic and rich-result appearance cannot be established by local
   unit tests or guaranteed by metadata changes.
5. For preview/tunnel/staging hosts, use the hosting platform's noindex response
   header or access restriction. Canonical URLs identify the production domain
   but do not prevent the host from being crawled by themselves.

References: [Google JavaScript SEO](https://developers.google.com/search/docs/crawling-indexing/javascript/javascript-seo-basics),
[noindex](https://developers.google.com/search/docs/crawling-indexing/block-indexing),
[localized URLs](https://developers.google.com/search/docs/specialty/international/localized-versions).

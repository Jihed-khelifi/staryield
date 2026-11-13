import { match } from "@formatjs/intl-localematcher"
import Negotiator from "negotiator"
import { NextResponse, type NextRequest } from "next/server"

import { defaultLocale, locales } from "@/i18n/config"
import { isRefundAlias } from "@/lib/legal/refund-aliases" // revisions 5.1

const LOCALE_COOKIE = "STARYIELD_LOCALE"

function getLocale(request: NextRequest) {
  const cookieLocale = request.cookies.get(LOCALE_COOKIE)?.value
  if (cookieLocale && (locales as readonly string[]).includes(cookieLocale)) {
    return cookieLocale
  }

  const acceptLanguage = request.headers.get("accept-language")
  if (!acceptLanguage) return defaultLocale

  const languages = new Negotiator({
    headers: { "accept-language": acceptLanguage },
  }).languages()

  try {
    return match(languages, locales as readonly string[], defaultLocale)
  } catch {
    return defaultLocale
  }
}

export function proxy(request: NextRequest) {
  const { pathname } = request.nextUrl

  // revisions 5.1: /refunds, /refund, /reimburse and /refunds-policy (with or without a locale)
  // go permanently (308) to /{locale}/refund-policy, in one hop (docs/platform-extensions.md §25.6).
  const segments = pathname.split("/").filter(Boolean)
  const explicit = segments.length === 2 && (locales as readonly string[]).includes(segments[0]) ? segments[0] : null
  if ((segments.length === 1 && isRefundAlias(segments[0])) || (explicit && isRefundAlias(segments[1]))) {
    const target = request.nextUrl.clone()
    target.pathname = `/${explicit ?? getLocale(request)}/refund-policy`
    return NextResponse.redirect(target, 308)
  }

  const pathnameHasLocale = locales.some(
    (locale) => pathname === `/${locale}` || pathname.startsWith(`/${locale}/`)
  )

  if (pathnameHasLocale) return

  const locale = getLocale(request)
  request.nextUrl.pathname = `/${locale}${pathname}`

  return NextResponse.redirect(request.nextUrl)
}

export const config = {
  // `/go/:code` short links are route handlers outside the locale tree.
  matcher: ["/((?!_next|api|assets|go/|favicon.ico|.*\\.[\\w]+$).*)"],
}

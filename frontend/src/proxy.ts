import { match } from "@formatjs/intl-localematcher"
import Negotiator from "negotiator"
import { NextResponse, type NextRequest } from "next/server"

import { defaultLocale, locales } from "@/i18n/config"

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

  const pathnameHasLocale = locales.some(
    (locale) => pathname === `/${locale}` || pathname.startsWith(`/${locale}/`)
  )

  if (pathnameHasLocale) return

  const locale = getLocale(request)
  request.nextUrl.pathname = `/${locale}${pathname}`

  return NextResponse.redirect(request.nextUrl)
}

export const config = {
  matcher: ["/((?!_next|api|assets|favicon.ico|.*\\.[\\w]+$).*)"],
}

import Home from "@/components/content/home";
import { PublicFrame } from "@/components/layout/public-frame";
import { localePath, locales } from "@/i18n/config";

export const metadata = {
  alternates: {
    languages: Object.fromEntries(
      locales.map((locale) => [locale, localePath("/", locale)]),
    ),
  },
};

export default function LangIndexPage() {
  return (
    <PublicFrame>
      <Home />
    </PublicFrame>
  );
}

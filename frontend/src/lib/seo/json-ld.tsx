import { serializeJsonLd, type JsonLdObject } from "@/lib/seo/jsonld";

/** Structured data for the page (server component; the data is escaped, see `serializeJsonLd`). */
export function JsonLd({ data }: { data: JsonLdObject | JsonLdObject[] }) {
  return <script type="application/ld+json" dangerouslySetInnerHTML={{ __html: serializeJsonLd(data) }} />;
}

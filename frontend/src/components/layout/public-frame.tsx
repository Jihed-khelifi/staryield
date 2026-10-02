import { SiteHeader } from "@/components/layout/site-header";
import { SiteFooter } from "@/components/layout/site-footer";
export function PublicFrame({
  children,
  promo = true,
}: {
  children: React.ReactNode;
  promo?: boolean;
}) {
  return (
    <div className="public-frame">
      <SiteHeader publicSite promo={promo} />
      <main>{children}</main>
      <SiteFooter />
    </div>
  );
}

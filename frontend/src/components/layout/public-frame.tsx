import { SiteHeader } from "@/components/layout/site-header";
import { SiteFooter } from "@/components/layout/site-footer";
export function PublicFrame({
  children,
  promo = true,
  mainClassName,
}: {
  children: React.ReactNode;
  promo?: boolean;
  /** Class of the page's <main> (a screen whose own layout lives on its main landmark). */
  mainClassName?: string;
}) {
  return (
    <div className="public-frame">
      <SiteHeader publicSite promo={promo} />
      <main className={mainClassName}>{children}</main>
      <SiteFooter />
    </div>
  );
}

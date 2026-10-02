"use client";
import { useI18n } from "@/i18n/i18n-provider";
/* Figma 764:1945 — reusable responsive content regions. */
/* eslint-disable @next/next/no-img-element */
import { ContentLink } from "@/components/content/content-link";
const assetPathPrefix = "https://assets.staryield.net/assets/figma";
const imgInstanceStar = `${assetPathPrefix}/7d7cc.png`;
const imgInstanceStar1 = `${assetPathPrefix}/146b1.png`;
const imgDividerHorizontal = `${assetPathPrefix}/a3b24.svg`;
export default function ScreenDesktopFaq() {
  const { text } = useI18n();
  return (
    <div
      className="figma-content flex w-full flex-col items-center bg-cream"
      data-node-id="764:1945"
      data-name="Screen / Desktop / FAQ"
    >
      <section
        className="bg-[var(--cream)] border-[var(--sage)] border-b border-solid content-stretch flex flex-col gap-[24px] items-center justify-center px-[120px] py-[96px] relative shrink-0 w-full figma-section"
        data-node-id="764:1975"
        data-name="Section / FAQ Hero"
      >
        <div
          className="content-stretch flex gap-[8px] items-center relative shrink-0"
          data-node-id="764:1976"
          data-name="Badge / STARYIELD COMPASS"
        >
          <div
            className="relative shrink-0 size-[18px]"
            data-node-id="764:1977"
            data-name="Instance / Star"
          >
            <div className="absolute inset-[-5.56%]">
              <img
                alt=""
                className="block max-w-none size-full"
                height="20"
                src={imgInstanceStar}
                width="20"
              />
            </div>
          </div>
          <p
            className="[word-break:break-word] font-display leading-[normal] not-italic relative shrink-0 text-[#1a1a1a] text-[14px] tracking-[0.42px] whitespace-nowrap"
            data-node-id="764:1979"
          >
            {text("STARYIELD COMPASS ")}
          </p>
          <div
            className="relative shrink-0 size-[18px]"
            data-node-id="764:1980"
            data-name="Instance / Star / 02"
          >
            <div className="absolute inset-[-5.56%]">
              <img
                alt=""
                className="block max-w-none size-full"
                height="20"
                src={imgInstanceStar}
                width="20"
              />
            </div>
          </div>
        </div>
        <h1
          className="[word-break:break-word] font-display leading-[normal] not-italic relative shrink-0 text-[#1a1a1a] text-[56px] text-center tracking-[2.24px] whitespace-nowrap"
          data-node-id="764:1982"
        >
          {text("FAQ ")}
        </h1>
        <p
          className="[word-break:break-word] font-serif font-light leading-[36px] relative shrink-0 text-[#1a1a1a] text-[24px] text-center w-[800px]"
          data-node-id="764:1983"
        >
          {text("Please find the answers to the most frequent questions here ")}
        </p>
        <div
          className="[word-break:break-word] content-stretch flex flex-col font-normal gap-[12px] items-center leading-[0] relative shrink-0 text-[#555] text-[16px] text-center"
          data-node-id="764:1984"
          data-name="Container / Hero Links Info"
        >
          <p
            className="font-serif italic relative shrink-0 w-[700px]"
            data-node-id="764:1985"
          >
            <span className="leading-[24px]">
              {text(
                `*If your questions stay unanswered, please contact us via `,
              )}
            </span>
            <ContentLink
              className="[text-decoration-skip-ink:none] [text-underline-position:from-font] cursor-pointer decoration-from-font decoration-solid leading-[24px] text-[#c29a3d] underline"
              href="https://staryield.com/support"
              target="_blank"
              label="support"
            >
              <span className="[text-decoration-skip-ink:none] [text-underline-position:from-font] decoration-from-font decoration-solid underline">
                {text("support ")}
              </span>
            </ContentLink>
            <span className="leading-[24px]">.</span>
          </p>
          <p
            className="font-serif relative shrink-0 w-[700px]"
            data-node-id="764:1986"
          >
            <span className="leading-[24px]">
              {text(
                `If you need help with your subscription, mobile or web app, please contact us `,
              )}
            </span>
            <ContentLink
              className="[text-decoration-skip-ink:none] [text-underline-position:from-font] cursor-pointer decoration-from-font decoration-solid leading-[24px] text-[#c29a3d] underline"
              href="https://staryield.com/support"
              target="_blank"
              label="here"
            >
              <span className="[text-decoration-skip-ink:none] [text-underline-position:from-font] decoration-from-font decoration-solid underline">
                {text("here ")}
              </span>
            </ContentLink>
          </p>
        </div>
      </section>
      <div
        className="bg-gradient-to-b border-[var(--sage)] border-b border-solid content-stretch flex flex-col from-[var(--cream)] gap-10 lg:gap-20 items-center justify-center px-[120px] py-14 lg:py-[100px] relative shrink-0 to-[var(--warm-gray)] w-full"
        data-node-id="764:1987"
        data-name="Container / FAQ Core Matrix"
      >
        <section
          className="content-stretch flex gap-[40px] items-start justify-center relative shrink-0 w-full figma-section"
          data-node-id="764:1988"
          data-name="Section / FAQ Block"
        >
          <section
            className="content-stretch flex flex-col items-start relative shrink-0 w-[455px] figma-section"
            data-node-id="764:1989"
            data-name="Section / Left Column"
          >
            <div
              className="content-stretch flex gap-[12px] items-center pb-[16px] relative shrink-0"
              data-node-id="764:1990"
              data-name="Header / Category"
            >
              <div
                className="relative shrink-0 size-[34.07px]"
                data-node-id="764:1991"
                data-name="Instance / Star"
              >
                <img
                  alt=""
                  className="absolute block inset-0 max-w-none size-full"
                  height="34.07"
                  src={imgInstanceStar1}
                  width="34.07"
                />
              </div>
              <p
                className="[word-break:break-word] font-display leading-[normal] not-italic relative shrink-0 text-[20px] text-black tracking-[0.6px] whitespace-nowrap"
                data-node-id="764:1993"
              >
                {text("Web subscription ")}
              </p>
            </div>
            <p
              className="[word-break:break-word] font-serif font-normal leading-[22px] min-w-full relative shrink-0 text-[14px] text-black w-[min-content]"
              data-node-id="764:1994"
            >
              {text(
                "Manage your billing settings, invoices, reports, and premium compatibility results generated through our portal. ",
              )}
            </p>
          </section>
          <section
            className="[word-break:break-word] content-stretch cursor-pointer flex flex-col gap-[16px] items-start relative shrink-0 text-black text-left w-[705px] whitespace-nowrap figma-section"
            data-node-id="764:1995"
            data-name="Section / Right Column"
          >
            <ContentLink
              className="content-stretch flex flex-col gap-[8px] items-start overflow-clip pb-[32px] relative shrink-0 w-[495px]"
              data-node-id="764:1996"
              data-name="Instance / State=Collapsed"
              label="↑ How can I cancel my staryield Website subscription? staryield WEB PORTAL PORTAL BILLING"
            >
              <p
                className="font-serif font-normal leading-[28px] relative shrink-0 text-[22px]"
                data-node-id="I764:1996;725:1130"
              >
                ↑
              </p>
              <div
                className="content-stretch flex flex-col gap-[2px] items-start leading-[normal] relative shrink-0 text-[9px] w-full"
                data-node-id="I764:1996;713:369"
                data-name="Frame"
              >
                <p
                  className="font-display not-italic relative shrink-0 tracking-[1.5px] uppercase"
                  data-node-id="I764:1996;713:370"
                >
                  {text("How can I cancel my staryield Website subscription? ")}
                </p>
                <p
                  className="font-serif font-normal relative shrink-0 tracking-[1px]"
                  data-node-id="I764:1996;713:371"
                >
                  {text("staryield WEB PORTAL ")}
                </p>
                <p
                  className="font-serif font-semibold relative shrink-0 tracking-[1.5px] uppercase"
                  data-node-id="I764:1996;713:372"
                >
                  {text("PORTAL BILLING ")}
                </p>
              </div>
            </ContentLink>
            <ContentLink
              className="content-stretch flex flex-col gap-[8px] items-start overflow-clip pb-[32px] relative shrink-0 w-[495px]"
              data-node-id="764:2003"
              data-name="Instance / State=Collapsed / 02"
              label="↑ What features are included in STARYIELD Website subscription? STARYIELD WEB PORTAL PLATFORM TIERS"
            >
              <p
                className="font-serif font-normal leading-[28px] relative shrink-0 text-[22px]"
                data-node-id="I764:2003;725:1130"
              >
                ↑
              </p>
              <div
                className="content-stretch flex flex-col gap-[2px] items-start leading-[normal] relative shrink-0 text-[9px] w-full"
                data-node-id="I764:2003;713:369"
                data-name="Frame"
              >
                <p
                  className="font-display not-italic relative shrink-0 tracking-[1.5px] uppercase"
                  data-node-id="I764:2003;713:370"
                >
                  {text(
                    "What features are included in STARYIELD Website subscription? ",
                  )}
                </p>
                <p
                  className="font-serif font-normal relative shrink-0 tracking-[1px]"
                  data-node-id="I764:2003;713:371"
                >
                  {text("STARYIELD WEB PORTAL ")}
                </p>
                <p
                  className="font-serif font-semibold relative shrink-0 tracking-[1.5px] uppercase"
                  data-node-id="I764:2003;713:372"
                >
                  {text("PLATFORM TIERS ")}
                </p>
              </div>
            </ContentLink>
            <ContentLink
              className="content-stretch flex flex-col gap-[8px] items-start overflow-clip pb-[32px] relative shrink-0 w-[495px]"
              data-node-id="764:2010"
              data-name="Instance / State=Collapsed / 03"
              label="↑ Did not receive the report ordered on social networks! DELIVERY & ACQUISITIONS SOCIAL COMMERCE"
            >
              <p
                className="font-serif font-normal leading-[28px] relative shrink-0 text-[22px]"
                data-node-id="I764:2010;725:1130"
              >
                ↑
              </p>
              <div
                className="content-stretch flex flex-col gap-[2px] items-start leading-[normal] relative shrink-0 text-[9px] w-full"
                data-node-id="I764:2010;713:369"
                data-name="Frame"
              >
                <p
                  className="font-display not-italic relative shrink-0 tracking-[1.5px] uppercase"
                  data-node-id="I764:2010;713:370"
                >
                  {text(
                    "Did not receive the report ordered on social networks! ",
                  )}
                </p>
                <p
                  className="font-serif font-normal relative shrink-0 tracking-[1px]"
                  data-node-id="I764:2010;713:371"
                >
                  {text(`DELIVERY & ACQUISITIONS`)}
                </p>
                <p
                  className="font-serif font-semibold relative shrink-0 tracking-[1.5px] uppercase"
                  data-node-id="I764:2010;713:372"
                >
                  {text("SOCIAL COMMERCE ")}
                </p>
              </div>
            </ContentLink>
            <ContentLink
              className="content-stretch flex flex-col gap-[8px] items-start overflow-clip pb-[32px] relative shrink-0 w-[621px]"
              data-node-id="764:2017"
              data-name="Instance / State=Collapsed / 04"
              label="↑ I made an error during the order for the Compatibility report on social networks! CORRECTION MATRIX REPORT ADJUSTMENT"
            >
              <p
                className="font-serif font-normal leading-[28px] relative shrink-0 text-[22px]"
                data-node-id="I764:2017;725:1130"
              >
                ↑
              </p>
              <div
                className="content-stretch flex flex-col gap-[2px] items-start leading-[normal] relative shrink-0 text-[9px] w-full"
                data-node-id="I764:2017;713:369"
                data-name="Frame"
              >
                <p
                  className="font-display not-italic relative shrink-0 tracking-[1.5px] uppercase"
                  data-node-id="I764:2017;713:370"
                >
                  {text(
                    "I made an error during the order for the Compatibility report on social networks! ",
                  )}
                </p>
                <p
                  className="font-serif font-normal relative shrink-0 tracking-[1px]"
                  data-node-id="I764:2017;713:371"
                >
                  {text("CORRECTION MATRIX ")}
                </p>
                <p
                  className="font-serif font-semibold relative shrink-0 tracking-[1.5px] uppercase"
                  data-node-id="I764:2017;713:372"
                >
                  {text("REPORT ADJUSTMENT ")}
                </p>
              </div>
            </ContentLink>
          </section>
        </section>
        <div
          className="h-0 relative shrink-0 w-full"
          data-node-id="764:2024"
          data-name="Divider / Horizontal"
        >
          <div className="absolute inset-[-1px_0_0_0]">
            <img
              alt=""
              className="block max-w-none size-full"
              src={imgDividerHorizontal}
            />
          </div>
        </div>
        <section
          className="content-stretch flex gap-[40px] items-start relative shrink-0 w-full figma-section"
          data-node-id="764:2025"
          data-name="Section / FAQ Block / 02"
        >
          <section
            className="content-stretch flex flex-col items-start relative shrink-0 w-[455px] figma-section"
            data-node-id="764:2026"
            data-name="Section / Left Column"
          >
            <div
              className="content-stretch flex gap-[12px] items-center pb-[16px] relative shrink-0"
              data-node-id="764:2027"
              data-name="Header / Category"
            >
              <div
                className="relative shrink-0 size-[34.07px]"
                data-node-id="764:2028"
                data-name="Instance / Star"
              >
                <img
                  alt=""
                  className="absolute block inset-0 max-w-none size-full"
                  height="34.07"
                  src={imgInstanceStar1}
                  width="34.07"
                />
              </div>
              <p
                className="[word-break:break-word] font-display leading-[normal] not-italic relative shrink-0 text-[20px] text-black tracking-[0.6px] whitespace-nowrap"
                data-node-id="764:2030"
              >
                {text("STARYIELD Website Functional ")}
              </p>
            </div>
            <p
              className="[word-break:break-word] font-serif font-normal leading-[22px] min-w-full relative shrink-0 text-[14px] text-black w-[min-content]"
              data-node-id="764:2031"
            >
              {text(
                "Resolve visual glitches, login validation hurdles, account deletions, and custom date configuration variables. ",
              )}
            </p>
          </section>
          <section
            className="[word-break:break-word] content-stretch cursor-pointer flex flex-col gap-[16px] items-start relative shrink-0 text-black text-left w-[705px] whitespace-nowrap figma-section"
            data-node-id="764:2032"
            data-name="Section / Right Column"
          >
            <ContentLink
              className="content-stretch flex flex-col gap-[8px] items-start overflow-clip pb-[32px] relative shrink-0 w-[495px]"
              data-node-id="764:2033"
              data-name="Instance / State=Collapsed"
              label="↑ Why is my astrological info wrong on STARYIELD Website? TECHNICAL EXECUTION DATA ANOMALY"
            >
              <p
                className="font-serif font-normal leading-[28px] relative shrink-0 text-[22px]"
                data-node-id="I764:2033;725:1130"
              >
                ↑
              </p>
              <div
                className="content-stretch flex flex-col gap-[2px] items-start leading-[normal] relative shrink-0 text-[9px] w-full"
                data-node-id="I764:2033;713:369"
                data-name="Frame"
              >
                <p
                  className="font-display not-italic relative shrink-0 tracking-[1.5px] uppercase"
                  data-node-id="I764:2033;713:370"
                >
                  {text(
                    "Why is my astrological info wrong on STARYIELD Website? ",
                  )}
                </p>
                <p
                  className="font-serif font-normal relative shrink-0 tracking-[1px]"
                  data-node-id="I764:2033;713:371"
                >
                  {text("TECHNICAL EXECUTION ")}
                </p>
                <p
                  className="font-serif font-semibold relative shrink-0 tracking-[1.5px] uppercase"
                  data-node-id="I764:2033;713:372"
                >
                  {text("DATA ANOMALY ")}
                </p>
              </div>
            </ContentLink>
            <ContentLink
              className="content-stretch flex flex-col gap-[8px] items-start overflow-clip pb-[32px] relative shrink-0 w-[495px]"
              data-node-id="764:2040"
              data-name="Instance / State=Collapsed / 02"
              label="↑ How can I log in to STARYIELD Website? TECHNICAL EXECUTION AUTHENTICATION"
            >
              <p
                className="font-serif font-normal leading-[28px] relative shrink-0 text-[22px]"
                data-node-id="I764:2040;725:1130"
              >
                ↑
              </p>
              <div
                className="content-stretch flex flex-col gap-[2px] items-start leading-[normal] relative shrink-0 text-[9px] w-full"
                data-node-id="I764:2040;713:369"
                data-name="Frame"
              >
                <p
                  className="font-display not-italic relative shrink-0 tracking-[1.5px] uppercase"
                  data-node-id="I764:2040;713:370"
                >
                  {text("How can I log in to STARYIELD Website? ")}
                </p>
                <p
                  className="font-serif font-normal relative shrink-0 tracking-[1px]"
                  data-node-id="I764:2040;713:371"
                >
                  {text("TECHNICAL EXECUTION ")}
                </p>
                <p
                  className="font-serif font-semibold relative shrink-0 tracking-[1.5px] uppercase"
                  data-node-id="I764:2040;713:372"
                >
                  {text("AUTHENTICATION ")}
                </p>
              </div>
            </ContentLink>
            <ContentLink
              className="content-stretch flex flex-col gap-[8px] items-start overflow-clip pb-[32px] relative shrink-0 w-[495px]"
              data-node-id="764:2047"
              data-name="Instance / State=Collapsed / 03"
              label="↑ Where can I find the settings on the STARYIELD Website? TECHNICAL EXECUTION DASHBOARD PREFERENCES"
            >
              <p
                className="font-serif font-normal leading-[28px] relative shrink-0 text-[22px]"
                data-node-id="I764:2047;725:1130"
              >
                ↑
              </p>
              <div
                className="content-stretch flex flex-col gap-[2px] items-start leading-[normal] relative shrink-0 text-[9px] w-full"
                data-node-id="I764:2047;713:369"
                data-name="Frame"
              >
                <p
                  className="font-display not-italic relative shrink-0 tracking-[1.5px] uppercase"
                  data-node-id="I764:2047;713:370"
                >
                  {text(
                    "Where can I find the settings on the STARYIELD Website? ",
                  )}
                </p>
                <p
                  className="font-serif font-normal relative shrink-0 tracking-[1px]"
                  data-node-id="I764:2047;713:371"
                >
                  {text("TECHNICAL EXECUTION ")}
                </p>
                <p
                  className="font-serif font-semibold relative shrink-0 tracking-[1.5px] uppercase"
                  data-node-id="I764:2047;713:372"
                >
                  {text("DASHBOARD PREFERENCES ")}
                </p>
              </div>
            </ContentLink>
            <ContentLink
              className="content-stretch flex flex-col gap-[8px] items-start overflow-clip pb-[32px] relative shrink-0 w-[495px]"
              data-node-id="764:2061"
              data-name="Instance / State=Collapsed / 04"
              label="↑ How to reset staryield Website account's password? TECHNICAL EXECUTION SECURITY"
            >
              <p
                className="font-serif font-normal leading-[28px] relative shrink-0 text-[22px]"
                data-node-id="I764:2061;725:1130"
              >
                ↑
              </p>
              <div
                className="content-stretch flex flex-col gap-[2px] items-start leading-[normal] relative shrink-0 text-[9px] w-full"
                data-node-id="I764:2061;713:369"
                data-name="Frame"
              >
                <p
                  className="font-display not-italic relative shrink-0 tracking-[1.5px] uppercase"
                  data-node-id="I764:2061;713:370"
                >
                  {text(`How to reset staryield Website account's password?`)}
                </p>
                <p
                  className="font-serif font-normal relative shrink-0 tracking-[1px]"
                  data-node-id="I764:2061;713:371"
                >
                  {text("TECHNICAL EXECUTION ")}
                </p>
                <p
                  className="font-serif font-semibold relative shrink-0 tracking-[1.5px] uppercase"
                  data-node-id="I764:2061;713:372"
                >
                  {text("SECURITY ")}
                </p>
              </div>
            </ContentLink>
            <ContentLink
              className="content-stretch flex flex-col gap-[8px] items-start overflow-clip pb-[32px] relative shrink-0 w-[495px]"
              data-node-id="764:2068"
              data-name="Instance / State=Collapsed / 05"
              label="↑ How to confirm STARYIELD Website account's email? TECHNICAL EXECUTION VALIDATION"
            >
              <p
                className="font-serif font-normal leading-[28px] relative shrink-0 text-[22px]"
                data-node-id="I764:2068;725:1130"
              >
                ↑
              </p>
              <div
                className="content-stretch flex flex-col gap-[2px] items-start leading-[normal] relative shrink-0 text-[9px] w-full"
                data-node-id="I764:2068;713:369"
                data-name="Frame"
              >
                <p
                  className="font-display not-italic relative shrink-0 tracking-[1.5px] uppercase"
                  data-node-id="I764:2068;713:370"
                >
                  {text(`How to confirm STARYIELD Website account's email?`)}
                </p>
                <p
                  className="font-serif font-normal relative shrink-0 tracking-[1px]"
                  data-node-id="I764:2068;713:371"
                >
                  {text("TECHNICAL EXECUTION ")}
                </p>
                <p
                  className="font-serif font-semibold relative shrink-0 tracking-[1.5px] uppercase"
                  data-node-id="I764:2068;713:372"
                >
                  {text("VALIDATION ")}
                </p>
              </div>
            </ContentLink>
            <ContentLink
              className="content-stretch flex flex-col gap-[8px] items-start overflow-clip pb-[32px] relative shrink-0 w-[495px]"
              data-node-id="764:2075"
              data-name="Instance / State=Collapsed / 06"
              label="↑ How can I change my personal data on STARYIELD Website? TECHNICAL EXECUTION PROFILE METADATA"
            >
              <p
                className="font-serif font-normal leading-[28px] relative shrink-0 text-[22px]"
                data-node-id="I764:2075;725:1130"
              >
                ↑
              </p>
              <div
                className="content-stretch flex flex-col gap-[2px] items-start leading-[normal] relative shrink-0 text-[9px] w-full"
                data-node-id="I764:2075;713:369"
                data-name="Frame"
              >
                <p
                  className="font-display not-italic relative shrink-0 tracking-[1.5px] uppercase"
                  data-node-id="I764:2075;713:370"
                >
                  {text(
                    "How can I change my personal data on STARYIELD Website? ",
                  )}
                </p>
                <p
                  className="font-serif font-normal relative shrink-0 tracking-[1px]"
                  data-node-id="I764:2075;713:371"
                >
                  {text("TECHNICAL EXECUTION ")}
                </p>
                <p
                  className="font-serif font-semibold relative shrink-0 tracking-[1.5px] uppercase"
                  data-node-id="I764:2075;713:372"
                >
                  {text("PROFILE METADATA ")}
                </p>
              </div>
            </ContentLink>
          </section>
        </section>
        <div
          className="h-0 relative shrink-0 w-full"
          data-node-id="764:2082"
          data-name="Divider / Horizontal / 02"
        >
          <div className="absolute inset-[-1px_0_0_0]">
            <img
              alt=""
              className="block max-w-none size-full"
              src={imgDividerHorizontal}
            />
          </div>
        </div>
        <section
          className="content-stretch flex gap-[40px] items-start relative shrink-0 w-full figma-section"
          data-node-id="764:2083"
          data-name="Section / FAQ Block / 03"
        >
          <section
            className="content-stretch flex flex-col items-start relative shrink-0 w-[455px] figma-section"
            data-node-id="764:2084"
            data-name="Section / Left Column"
          >
            <div
              className="content-stretch flex gap-[12px] items-center pb-[16px] relative shrink-0"
              data-node-id="764:2085"
              data-name="Header / Category"
            >
              <div
                className="relative shrink-0 size-[34.07px]"
                data-node-id="764:2086"
                data-name="Instance / Star"
              >
                <img
                  alt=""
                  className="absolute block inset-0 max-w-none size-full"
                  height="34.07"
                  src={imgInstanceStar1}
                  width="34.07"
                />
              </div>
              <div
                className="[word-break:break-word] font-display leading-[0] not-italic relative shrink-0 text-[20px] text-black tracking-[0.6px] whitespace-nowrap"
                data-node-id="764:2088"
              >
                <p className="leading-[normal] mb-0 whitespace-pre">
                  {text(`STARYIELD WEB `)}
                </p>
                <p className="leading-[normal] whitespace-pre">
                  {text("Chatting with Advisors ")}
                </p>
              </div>
            </div>
            <p
              className="[word-break:break-word] font-serif font-normal leading-[22px] min-w-full relative shrink-0 text-[14px] text-black w-[min-content]"
              data-node-id="764:2089"
            >
              {text(
                "How to establish deep, confidential consultations with our star specialist advisors without losing transactional integrity. ",
              )}
            </p>
          </section>
          <section
            className="[word-break:break-word] content-stretch cursor-pointer flex flex-col gap-[16px] items-start relative shrink-0 text-black text-left w-[705px] whitespace-nowrap figma-section"
            data-node-id="764:2090"
            data-name="Section / Right Column"
          >
            <ContentLink
              className="content-stretch flex flex-col gap-[8px] items-start pb-[32px] pt-[28px] relative shrink-0 w-full"
              data-node-id="764:2091"
              data-name="Instance / State=Collapsed"
              label="☉ I didn’t get a response from an astrologer/I was not satisfied with the reading I got. CONSULTATION PROTOCOLS QUALITY ASSURANCE"
            >
              <p
                className="font-serif font-normal leading-[28px] relative shrink-0 text-[22px]"
                data-node-id="I764:2091;713:386"
              >
                ☉
              </p>
              <div
                className="content-stretch flex flex-col gap-[2px] items-start leading-[normal] relative shrink-0 text-[9px] w-full"
                data-node-id="I764:2091;713:387"
                data-name="Frame"
              >
                <p
                  className="font-display not-italic relative shrink-0 tracking-[1.5px] uppercase"
                  data-node-id="I764:2091;713:388"
                >
                  {text(
                    "I didn’t get a response from an astrologer/I was not satisfied with the reading I got. ",
                  )}
                </p>
                <p
                  className="font-serif font-normal relative shrink-0 tracking-[1px]"
                  data-node-id="I764:2091;713:389"
                >
                  {text("CONSULTATION PROTOCOLS ")}
                </p>
                <p
                  className="font-serif font-semibold relative shrink-0 tracking-[1.5px] uppercase"
                  data-node-id="I764:2091;713:390"
                >
                  {text("QUALITY ASSURANCE ")}
                </p>
              </div>
            </ContentLink>
            <ContentLink
              className="content-stretch flex flex-col gap-[8px] items-start pb-[32px] pt-[28px] relative shrink-0 w-[495px]"
              data-node-id="764:2098"
              data-name="Instance / State=Collapsed / 02"
              label="☉ How can I choose the right advisor? CONSULTATION PROTOCOLS SPECIALIST MATCHING"
            >
              <p
                className="font-serif font-normal leading-[28px] relative shrink-0 text-[22px]"
                data-node-id="I764:2098;713:386"
              >
                ☉
              </p>
              <div
                className="content-stretch flex flex-col gap-[2px] items-start leading-[normal] relative shrink-0 text-[9px] w-full"
                data-node-id="I764:2098;713:387"
                data-name="Frame"
              >
                <p
                  className="font-display not-italic relative shrink-0 tracking-[1.5px] uppercase"
                  data-node-id="I764:2098;713:388"
                >
                  {text("How can I choose the right advisor? ")}
                </p>
                <p
                  className="font-serif font-normal relative shrink-0 tracking-[1px]"
                  data-node-id="I764:2098;713:389"
                >
                  {text("CONSULTATION PROTOCOLS ")}
                </p>
                <p
                  className="font-serif font-semibold relative shrink-0 tracking-[1.5px] uppercase"
                  data-node-id="I764:2098;713:390"
                >
                  {text("SPECIALIST MATCHING ")}
                </p>
              </div>
            </ContentLink>
            <ContentLink
              className="content-stretch flex flex-col gap-[8px] items-start pb-[32px] pt-[28px] relative shrink-0 w-[495px]"
              data-node-id="764:2105"
              data-name="Instance / State=Collapsed / 03"
              label="☉ Why did my minutes run out so fast on staryield Website? CONSULTATION PROTOCOLS TRANSIT CHRONOLOGY"
            >
              <p
                className="font-serif font-normal leading-[28px] relative shrink-0 text-[22px]"
                data-node-id="I764:2105;713:386"
              >
                ☉
              </p>
              <div
                className="content-stretch flex flex-col gap-[2px] items-start leading-[normal] relative shrink-0 text-[9px] w-full"
                data-node-id="I764:2105;713:387"
                data-name="Frame"
              >
                <p
                  className="font-display not-italic relative shrink-0 tracking-[1.5px] uppercase"
                  data-node-id="I764:2105;713:388"
                >
                  {text(
                    "Why did my minutes run out so fast on staryield Website? ",
                  )}
                </p>
                <p
                  className="font-serif font-normal relative shrink-0 tracking-[1px]"
                  data-node-id="I764:2105;713:389"
                >
                  {text("CONSULTATION PROTOCOLS ")}
                </p>
                <p
                  className="font-serif font-semibold relative shrink-0 tracking-[1.5px] uppercase"
                  data-node-id="I764:2105;713:390"
                >
                  {text("TRANSIT CHRONOLOGY ")}
                </p>
              </div>
            </ContentLink>
            <ContentLink
              className="content-stretch flex flex-col gap-[8px] items-start pb-[32px] pt-[28px] relative shrink-0 w-[495px]"
              data-node-id="764:2112"
              data-name="Instance / State=Collapsed / 04"
              label="☉ Why I can not send my question in chat on staryield Website? CONSULTATION PROTOCOLS PORTAL GLITCHES"
            >
              <p
                className="font-serif font-normal leading-[28px] relative shrink-0 text-[22px]"
                data-node-id="I764:2112;713:386"
              >
                ☉
              </p>
              <div
                className="content-stretch flex flex-col gap-[2px] items-start leading-[normal] relative shrink-0 text-[9px] w-full"
                data-node-id="I764:2112;713:387"
                data-name="Frame"
              >
                <p
                  className="font-display not-italic relative shrink-0 tracking-[1.5px] uppercase"
                  data-node-id="I764:2112;713:388"
                >
                  {text(
                    "Why I can not send my question in chat on staryield Website? ",
                  )}
                </p>
                <p
                  className="font-serif font-normal relative shrink-0 tracking-[1px]"
                  data-node-id="I764:2112;713:389"
                >
                  {text("CONSULTATION PROTOCOLS ")}
                </p>
                <p
                  className="font-serif font-semibold relative shrink-0 tracking-[1.5px] uppercase"
                  data-node-id="I764:2112;713:390"
                >
                  {text("PORTAL GLITCHES ")}
                </p>
              </div>
            </ContentLink>
            <ContentLink
              className="content-stretch flex flex-col gap-[8px] items-start pb-[32px] pt-[28px] relative shrink-0 w-[495px]"
              data-node-id="764:2119"
              data-name="Instance / State=Collapsed / 05"
              label="☉ Where can I find my chat/answer on staryield Website? CONSULTATION PROTOCOLS TRANSCRIPTION RECORD"
            >
              <p
                className="font-serif font-normal leading-[28px] relative shrink-0 text-[22px]"
                data-node-id="I764:2119;713:386"
              >
                ☉
              </p>
              <div
                className="content-stretch flex flex-col gap-[2px] items-start leading-[normal] relative shrink-0 text-[9px] w-full"
                data-node-id="I764:2119;713:387"
                data-name="Frame"
              >
                <p
                  className="font-display not-italic relative shrink-0 tracking-[1.5px] uppercase"
                  data-node-id="I764:2119;713:388"
                >
                  {text(
                    "Where can I find my chat/answer on staryield Website? ",
                  )}
                </p>
                <p
                  className="font-serif font-normal relative shrink-0 tracking-[1px]"
                  data-node-id="I764:2119;713:389"
                >
                  {text("CONSULTATION PROTOCOLS ")}
                </p>
                <p
                  className="font-serif font-semibold relative shrink-0 tracking-[1.5px] uppercase"
                  data-node-id="I764:2119;713:390"
                >
                  {text("TRANSCRIPTION RECORD ")}
                </p>
              </div>
            </ContentLink>
            <ContentLink
              className="content-stretch flex flex-col gap-[8px] items-start pb-[32px] pt-[28px] relative shrink-0 w-[495px]"
              data-node-id="764:2126"
              data-name="Instance / State=Collapsed / 06"
              label="☉ How can I use credits for chatting with advisors on staryield Website? CONSULTATION PROTOCOLS TRANSACTION MANAGEMENT"
            >
              <p
                className="font-serif font-normal leading-[28px] relative shrink-0 text-[22px]"
                data-node-id="I764:2126;713:386"
              >
                ☉
              </p>
              <div
                className="content-stretch flex flex-col gap-[2px] items-start leading-[normal] relative shrink-0 text-[9px] w-full"
                data-node-id="I764:2126;713:387"
                data-name="Frame"
              >
                <p
                  className="font-display not-italic relative shrink-0 tracking-[1.5px] uppercase"
                  data-node-id="I764:2126;713:388"
                >
                  {text(
                    "How can I use credits for chatting with advisors on staryield Website? ",
                  )}
                </p>
                <p
                  className="font-serif font-normal relative shrink-0 tracking-[1px]"
                  data-node-id="I764:2126;713:389"
                >
                  {text("CONSULTATION PROTOCOLS ")}
                </p>
                <p
                  className="font-serif font-semibold relative shrink-0 tracking-[1.5px] uppercase"
                  data-node-id="I764:2126;713:390"
                >
                  {text("TRANSACTION MANAGEMENT ")}
                </p>
              </div>
            </ContentLink>
          </section>
        </section>
        <div
          className="h-0 relative shrink-0 w-full"
          data-node-id="764:2133"
          data-name="Divider / Horizontal / 03"
        >
          <div className="absolute inset-[-1px_0_0_0]">
            <img
              alt=""
              className="block max-w-none size-full"
              src={imgDividerHorizontal}
            />
          </div>
        </div>
        <section
          className="content-stretch flex gap-[40px] items-start relative shrink-0 w-full figma-section"
          data-node-id="764:2134"
          data-name="Section / FAQ Block / 04"
        >
          <section
            className="content-stretch flex flex-col items-start relative shrink-0 w-[455px] figma-section"
            data-node-id="764:2135"
            data-name="Section / Left Column"
          >
            <div
              className="content-stretch flex gap-[12px] items-center pb-[16px] relative shrink-0"
              data-node-id="764:2136"
              data-name="Header / Category"
            >
              <div
                className="relative shrink-0 size-[34.07px]"
                data-node-id="764:2137"
                data-name="Instance / Star"
              >
                <img
                  alt=""
                  className="absolute block inset-0 max-w-none size-full"
                  height="34.07"
                  src={imgInstanceStar1}
                  width="34.07"
                />
              </div>
              <p
                className="[word-break:break-word] font-display leading-[normal] not-italic relative shrink-0 text-[20px] text-black tracking-[0.6px] whitespace-nowrap"
                data-node-id="764:2139"
              >
                {text("STARYIELD App Functionality ")}
              </p>
            </div>
            <p
              className="[word-break:break-word] font-serif font-normal leading-[22px] min-w-full relative shrink-0 text-[14px] text-black w-[min-content]"
              data-node-id="764:2140"
            >
              {text(
                "Find answers regarding native application settings, localization variables, alerts, and platform transfers. ",
              )}
            </p>
          </section>
          <section
            className="[word-break:break-word] content-stretch flex flex-col gap-[16px] items-start relative shrink-0 w-[705px] figma-section"
            data-node-id="764:2141"
            data-name="Section / Right Column"
          >
            <div
              className="content-stretch flex flex-col gap-[8px] items-start pb-[32px] pt-[28px] relative shrink-0 text-black w-[495px] whitespace-nowrap"
              data-node-id="764:2142"
              data-name="Instance / State=Collapsed"
            >
              <p
                className="font-serif font-normal leading-[28px] relative shrink-0 text-[22px]"
                data-node-id="I764:2142;713:403"
              >
                ☽
              </p>
              <div
                className="content-stretch flex flex-col gap-[2px] items-start leading-[normal] relative shrink-0 text-[9px] w-full"
                data-node-id="I764:2142;713:404"
                data-name="Frame"
              >
                <p
                  className="font-display not-italic relative shrink-0 tracking-[1.5px] uppercase"
                  data-node-id="I764:2142;713:405"
                >
                  {text("How can I change the language of the STARYIELD? ")}
                </p>
                <p
                  className="font-serif font-normal relative shrink-0 tracking-[1px]"
                  data-node-id="I764:2142;713:406"
                >
                  {text("NATIVE COMPILATION ")}
                </p>
                <p
                  className="font-serif font-semibold relative shrink-0 tracking-[1.5px] uppercase"
                  data-node-id="I764:2142;713:407"
                >
                  {text("LOCALIZATION ")}
                </p>
              </div>
            </div>
            <div
              className="content-stretch flex flex-col gap-[8px] items-start pb-[32px] pt-[28px] relative shrink-0 w-[495px]"
              data-node-id="764:2149"
              data-name="Instance / State=Collapsed / 02"
            >
              <p
                className="font-serif font-normal leading-[28px] relative shrink-0 text-[22px] text-black whitespace-nowrap"
                data-node-id="I764:2149;713:403"
              >
                ☽
              </p>
              <div
                className="content-stretch flex flex-col gap-[2px] items-start leading-[normal] relative shrink-0 text-[9px] text-black w-full whitespace-nowrap"
                data-node-id="I764:2149;713:404"
                data-name="Frame"
              >
                <p
                  className="font-display not-italic relative shrink-0 tracking-[1.5px] uppercase"
                  data-node-id="I764:2149;713:405"
                >
                  {text("How can I edit my profile in the STARYIELD App? ")}
                </p>
                <p
                  className="font-serif font-normal relative shrink-0 tracking-[1px]"
                  data-node-id="I764:2149;713:406"
                >
                  {text("NATIVE COMPILATION ")}
                </p>
                <p
                  className="font-serif font-semibold relative shrink-0 tracking-[1.5px] uppercase"
                  data-node-id="I764:2149;713:407"
                >
                  {text("DASHBOARD ")}
                </p>
              </div>
              <p
                className="font-serif font-normal leading-[20px] min-w-full relative shrink-0 text-[#1a1a1a] text-[13px] w-[min-content]"
                data-node-id="I764:2149;713:408"
              >
                {text(
                  "To edit your profile in the STARYIELD app, open the app, tap your profile photo, then tap Edit Profile. Update your details and save your changes. ",
                )}
              </p>
            </div>
            <div
              className="content-stretch flex flex-col gap-[8px] items-start pb-[32px] pt-[28px] relative shrink-0 text-black w-[495px] whitespace-nowrap"
              data-node-id="764:2156"
              data-name="Instance / State=Collapsed / 03"
            >
              <p
                className="font-serif font-normal leading-[28px] relative shrink-0 text-[22px]"
                data-node-id="I764:2156;713:403"
              >
                ☽
              </p>
              <div
                className="content-stretch flex flex-col gap-[2px] items-start leading-[normal] relative shrink-0 text-[9px] w-full"
                data-node-id="I764:2156;713:404"
                data-name="Frame"
              >
                <p
                  className="font-display not-italic relative shrink-0 tracking-[1.5px] uppercase"
                  data-node-id="I764:2156;713:405"
                >
                  {text(
                    "How can I transfer my STARYIELD account from one device to another? ",
                  )}
                </p>
                <p
                  className="font-serif font-normal relative shrink-0 tracking-[1px]"
                  data-node-id="I764:2156;713:406"
                >
                  {text("NATIVE COMPILATION ")}
                </p>
                <p
                  className="font-serif font-semibold relative shrink-0 tracking-[1.5px] uppercase"
                  data-node-id="I764:2156;713:407"
                >
                  {text("TRAJECTORY MIGRATION ")}
                </p>
              </div>
            </div>
            <div
              className="content-stretch flex flex-col gap-[8px] items-start pb-[32px] pt-[28px] relative shrink-0 text-black w-[495px] whitespace-nowrap"
              data-node-id="764:2163"
              data-name="Instance / State=Collapsed / 04"
            >
              <p
                className="font-serif font-normal leading-[28px] relative shrink-0 text-[22px]"
                data-node-id="I764:2163;713:403"
              >
                ☽
              </p>
              <div
                className="content-stretch flex flex-col gap-[2px] items-start leading-[normal] relative shrink-0 text-[9px] w-full"
                data-node-id="I764:2163;713:404"
                data-name="Frame"
              >
                <p
                  className="font-display not-italic relative shrink-0 tracking-[1.5px] uppercase"
                  data-node-id="I764:2163;713:405"
                >
                  {text(
                    "My device is not compatible with the STARYIELD or it is unavailable in my region. ",
                  )}
                </p>
                <p
                  className="font-serif font-normal relative shrink-0 tracking-[1px]"
                  data-node-id="I764:2163;713:406"
                >
                  {text("NATIVE COMPILATION ")}
                </p>
                <p
                  className="font-serif font-semibold relative shrink-0 tracking-[1.5px] uppercase"
                  data-node-id="I764:2163;713:407"
                >
                  {text("REGIONAL MATRIX ")}
                </p>
              </div>
            </div>
            <div
              className="content-stretch flex flex-col gap-[8px] items-start pb-[32px] pt-[28px] relative shrink-0 text-black w-[495px] whitespace-nowrap"
              data-node-id="764:2170"
              data-name="Instance / State=Collapsed / 05"
            >
              <p
                className="font-serif font-normal leading-[28px] relative shrink-0 text-[22px]"
                data-node-id="I764:2170;713:403"
              >
                ☽
              </p>
              <div
                className="content-stretch flex flex-col gap-[2px] items-start leading-[normal] relative shrink-0 text-[9px] w-full"
                data-node-id="I764:2170;713:404"
                data-name="Frame"
              >
                <p
                  className="font-display not-italic relative shrink-0 tracking-[1.5px] uppercase"
                  data-node-id="I764:2170;713:405"
                >
                  {text("How can I delete my profile in STARYIELD? ")}
                </p>
                <p
                  className="font-serif font-normal relative shrink-0 tracking-[1px]"
                  data-node-id="I764:2170;713:406"
                >
                  {text("NATIVE COMPILATION ")}
                </p>
                <p
                  className="font-serif font-semibold relative shrink-0 tracking-[1.5px] uppercase"
                  data-node-id="I764:2170;713:407"
                >
                  {text("DESTRUCTION ")}
                </p>
              </div>
            </div>
            <div
              className="content-stretch flex flex-col gap-[8px] items-start pb-[32px] pt-[28px] relative shrink-0 text-black w-[495px] whitespace-nowrap"
              data-node-id="764:2177"
              data-name="Instance / State=Collapsed / 06"
            >
              <p
                className="font-serif font-normal leading-[28px] relative shrink-0 text-[22px]"
                data-node-id="I764:2177;713:403"
              >
                ☽
              </p>
              <div
                className="content-stretch flex flex-col gap-[2px] items-start leading-[normal] relative shrink-0 text-[9px] w-full"
                data-node-id="I764:2177;713:404"
                data-name="Frame"
              >
                <p
                  className="font-display not-italic relative shrink-0 tracking-[1.5px] uppercase"
                  data-node-id="I764:2177;713:405"
                >
                  {text("What are the age restrictions for STARYIELD? ")}
                </p>
                <p
                  className="font-serif font-normal relative shrink-0 tracking-[1px]"
                  data-node-id="I764:2177;713:406"
                >
                  {text("NATIVE COMPILATION ")}
                </p>
                <p
                  className="font-serif font-semibold relative shrink-0 tracking-[1.5px] uppercase"
                  data-node-id="I764:2177;713:407"
                >
                  {text("REGULATORY COMPLIANCE ")}
                </p>
              </div>
            </div>
            <div
              className="content-stretch flex flex-col gap-[8px] items-start pb-[32px] pt-[28px] relative shrink-0 text-black w-[495px] whitespace-nowrap"
              data-node-id="764:2184"
              data-name="Instance / State=Collapsed / 07"
            >
              <p
                className="font-serif font-normal leading-[28px] relative shrink-0 text-[22px]"
                data-node-id="I764:2184;713:403"
              >
                ☽
              </p>
              <div
                className="content-stretch flex flex-col gap-[2px] items-start leading-[normal] relative shrink-0 text-[9px] w-full"
                data-node-id="I764:2184;713:404"
                data-name="Frame"
              >
                <p
                  className="font-display not-italic relative shrink-0 tracking-[1.5px] uppercase"
                  data-node-id="I764:2184;713:405"
                >
                  {text(
                    "I am facing issues with STARYIELD app, it is not working correctly. ",
                  )}
                </p>
                <p
                  className="font-serif font-normal relative shrink-0 tracking-[1px]"
                  data-node-id="I764:2184;713:406"
                >
                  {text("NATIVE COMPILATION ")}
                </p>
                <p
                  className="font-serif font-semibold relative shrink-0 tracking-[1.5px] uppercase"
                  data-node-id="I764:2184;713:407"
                >
                  {text("SYSTEM AUDIT ")}
                </p>
              </div>
            </div>
            <div
              className="content-stretch flex flex-col gap-[8px] items-start pb-[32px] pt-[28px] relative shrink-0 text-black w-[495px] whitespace-nowrap"
              data-node-id="764:2191"
              data-name="Instance / State=Collapsed / 08"
            >
              <p
                className="font-serif font-normal leading-[28px] relative shrink-0 text-[22px]"
                data-node-id="I764:2191;713:403"
              >
                ☽
              </p>
              <div
                className="content-stretch flex flex-col gap-[2px] items-start leading-[normal] relative shrink-0 text-[9px] w-full"
                data-node-id="I764:2191;713:404"
                data-name="Frame"
              >
                <p
                  className="font-display not-italic relative shrink-0 tracking-[1.5px] uppercase"
                  data-node-id="I764:2191;713:405"
                >
                  {text(
                    "My Zodiac/Ascendant sign is not displayed correctly in the STARYIELD app ",
                  )}
                </p>
                <p
                  className="font-serif font-normal relative shrink-0 tracking-[1px]"
                  data-node-id="I764:2191;713:406"
                >
                  {text("NATIVE COMPILATION ")}
                </p>
                <p
                  className="font-serif font-semibold relative shrink-0 tracking-[1.5px] uppercase"
                  data-node-id="I764:2191;713:407"
                >
                  {text("ALIGNMENT ANOMALY ")}
                </p>
              </div>
            </div>
            <div
              className="content-stretch flex flex-col gap-[8px] items-start pb-[32px] pt-[28px] relative shrink-0 text-black w-[495px] whitespace-nowrap"
              data-node-id="764:2198"
              data-name="Instance / State=Collapsed / 09"
            >
              <p
                className="font-serif font-normal leading-[28px] relative shrink-0 text-[22px]"
                data-node-id="I764:2198;713:403"
              >
                ☽
              </p>
              <div
                className="content-stretch flex flex-col gap-[2px] items-start leading-[normal] relative shrink-0 text-[9px] w-full"
                data-node-id="I764:2198;713:404"
                data-name="Frame"
              >
                <p
                  className="font-display not-italic relative shrink-0 tracking-[1.5px] uppercase"
                  data-node-id="I764:2198;713:405"
                >
                  {text("How to turn off the notifications? ")}
                </p>
                <p
                  className="font-serif font-normal relative shrink-0 tracking-[1px]"
                  data-node-id="I764:2198;713:406"
                >
                  {text("NATIVE COMPILATION ")}
                </p>
                <p
                  className="font-serif font-semibold relative shrink-0 tracking-[1.5px] uppercase"
                  data-node-id="I764:2198;713:407"
                >
                  {text("ALERTS ")}
                </p>
              </div>
            </div>
            <div
              className="content-stretch flex flex-col gap-[8px] items-start pb-[32px] pt-[28px] relative shrink-0 text-black w-[495px] whitespace-nowrap"
              data-node-id="764:2205"
              data-name="Instance / State=Collapsed / 10"
            >
              <p
                className="font-serif font-normal leading-[28px] relative shrink-0 text-[22px]"
                data-node-id="I764:2205;713:403"
              >
                ☽
              </p>
              <div
                className="content-stretch flex flex-col gap-[2px] items-start leading-[normal] relative shrink-0 text-[9px] w-full"
                data-node-id="I764:2205;713:404"
                data-name="Frame"
              >
                <p
                  className="font-display not-italic relative shrink-0 tracking-[1.5px] uppercase"
                  data-node-id="I764:2205;713:405"
                >
                  {text(`How can I change my Partner's data inside the App?`)}
                </p>
                <p
                  className="font-serif font-normal relative shrink-0 tracking-[1px]"
                  data-node-id="I764:2205;713:406"
                >
                  {text("NATIVE COMPILATION ")}
                </p>
                <p
                  className="font-serif font-semibold relative shrink-0 tracking-[1.5px] uppercase"
                  data-node-id="I764:2205;713:407"
                >
                  {text("COMPATIBILITY SCHEMA ")}
                </p>
              </div>
            </div>
            <div
              className="content-stretch flex flex-col gap-[8px] items-start pb-[32px] pt-[28px] relative shrink-0 text-black w-[495px] whitespace-nowrap"
              data-node-id="764:2212"
              data-name="Instance / State=Collapsed / 11"
            >
              <p
                className="font-serif font-normal leading-[28px] relative shrink-0 text-[22px]"
                data-node-id="I764:2212;713:403"
              >
                ☽
              </p>
              <div
                className="content-stretch flex flex-col gap-[2px] items-start leading-[normal] relative shrink-0 text-[9px] w-full"
                data-node-id="I764:2212;713:404"
                data-name="Frame"
              >
                <p
                  className="font-display not-italic relative shrink-0 tracking-[1.5px] uppercase"
                  data-node-id="I764:2212;713:405"
                >
                  {text("How can I use STARYIELD for free? ")}
                </p>
                <p
                  className="font-serif font-normal relative shrink-0 tracking-[1px]"
                  data-node-id="I764:2212;713:406"
                >
                  {text("NATIVE COMPILATION ")}
                </p>
                <p
                  className="font-serif font-semibold relative shrink-0 tracking-[1.5px] uppercase"
                  data-node-id="I764:2212;713:407"
                >
                  {text("CELESTIAL SERVICES ")}
                </p>
              </div>
            </div>
          </section>
        </section>
        <div
          className="h-0 relative shrink-0 w-full"
          data-node-id="764:2219"
          data-name="Divider / Horizontal / 04"
        >
          <div className="absolute inset-[-1px_0_0_0]">
            <img
              alt=""
              className="block max-w-none size-full"
              src={imgDividerHorizontal}
            />
          </div>
        </div>
        <section
          className="content-stretch flex gap-[40px] items-start relative shrink-0 w-full figma-section"
          data-node-id="764:2220"
          data-name="Section / FAQ Block / 05"
        >
          <section
            className="content-stretch flex flex-col items-start relative shrink-0 w-[455px] figma-section"
            data-node-id="764:2221"
            data-name="Section / Left Column"
          >
            <div
              className="content-stretch flex gap-[12px] items-center pb-[16px] relative shrink-0"
              data-node-id="764:2222"
              data-name="Header / Category"
            >
              <div
                className="relative shrink-0 size-[34.07px]"
                data-node-id="764:2223"
                data-name="Instance / Star"
              >
                <img
                  alt=""
                  className="absolute block inset-0 max-w-none size-full"
                  height="34.07"
                  src={imgInstanceStar1}
                  width="34.07"
                />
              </div>
              <p
                className="[word-break:break-word] font-display leading-[normal] not-italic relative shrink-0 text-[20px] text-black tracking-[0.6px] whitespace-nowrap"
                data-node-id="764:2225"
              >
                {text("STARYIELD Premium Features ")}
              </p>
            </div>
            <p
              className="[word-break:break-word] font-serif font-normal leading-[22px] min-w-full relative shrink-0 text-[14px] text-black w-[min-content]"
              data-node-id="764:2226"
            >
              {text(
                "Everything about Premium capabilities, reports delivery errors, SpinWheel adjustments, and multi-device cross-platform restores. ",
              )}
            </p>
          </section>
          <section
            className="[word-break:break-word] content-stretch cursor-pointer flex flex-col gap-[16px] items-start relative shrink-0 text-black text-left w-[705px] whitespace-nowrap figma-section"
            data-node-id="764:2227"
            data-name="Section / Right Column"
          >
            <ContentLink
              className="content-stretch flex flex-col gap-[8px] items-start overflow-clip pb-[32px] relative shrink-0 w-[495px]"
              data-node-id="764:2228"
              data-name="Instance / State=Collapsed"
              label="↑ What are the benefits of a Premium subscription in STARYIELD app? PREMIUM FUNCTIONALITY PREMIUM TIERS"
            >
              <p
                className="font-serif font-normal leading-[28px] relative shrink-0 text-[22px]"
                data-node-id="I764:2228;725:1130"
              >
                ↑
              </p>
              <div
                className="content-stretch flex flex-col gap-[2px] items-start leading-[normal] relative shrink-0 text-[9px] w-full"
                data-node-id="I764:2228;713:369"
                data-name="Frame"
              >
                <p
                  className="font-display not-italic relative shrink-0 tracking-[1.5px] uppercase"
                  data-node-id="I764:2228;713:370"
                >
                  {text(
                    "What are the benefits of a Premium subscription in STARYIELD app? ",
                  )}
                </p>
                <p
                  className="font-serif font-normal relative shrink-0 tracking-[1px]"
                  data-node-id="I764:2228;713:371"
                >
                  {text("PREMIUM FUNCTIONALITY ")}
                </p>
                <p
                  className="font-serif font-semibold relative shrink-0 tracking-[1.5px] uppercase"
                  data-node-id="I764:2228;713:372"
                >
                  {text("PREMIUM TIERS ")}
                </p>
              </div>
            </ContentLink>
            <ContentLink
              className="content-stretch flex flex-col gap-[8px] items-start overflow-clip pb-[32px] relative shrink-0 w-[495px]"
              data-node-id="764:2235"
              data-name="Instance / State=Collapsed / 02"
              label="↑ How can I transfer my STARYIELD subscription or account from iOS to Android and vice versa? PREMIUM FUNCTIONALITY PLATFORM MIGRATION"
            >
              <p
                className="font-serif font-normal leading-[28px] relative shrink-0 text-[22px]"
                data-node-id="I764:2235;725:1130"
              >
                ↑
              </p>
              <div
                className="content-stretch flex flex-col gap-[2px] items-start leading-[normal] relative shrink-0 text-[9px] w-full"
                data-node-id="I764:2235;713:369"
                data-name="Frame"
              >
                <p
                  className="font-display not-italic relative shrink-0 tracking-[1.5px] uppercase"
                  data-node-id="I764:2235;713:370"
                >
                  {text(
                    "How can I transfer my STARYIELD subscription or account from iOS to Android and vice versa? ",
                  )}
                </p>
                <p
                  className="font-serif font-normal relative shrink-0 tracking-[1px]"
                  data-node-id="I764:2235;713:371"
                >
                  {text("PREMIUM FUNCTIONALITY ")}
                </p>
                <p
                  className="font-serif font-semibold relative shrink-0 tracking-[1.5px] uppercase"
                  data-node-id="I764:2235;713:372"
                >
                  {text("PLATFORM MIGRATION ")}
                </p>
              </div>
            </ContentLink>
            <ContentLink
              className="content-stretch flex flex-col gap-[8px] items-start overflow-clip pb-[32px] relative shrink-0 w-[495px]"
              data-node-id="764:2242"
              data-name="Instance / State=Collapsed / 03"
              label="↑ I made an error during the order for the Compatibility report in STARYIELD. PREMIUM FUNCTIONALITY INTEGRITY RE-AUDIT"
            >
              <p
                className="font-serif font-normal leading-[28px] relative shrink-0 text-[22px]"
                data-node-id="I764:2242;725:1130"
              >
                ↑
              </p>
              <div
                className="content-stretch flex flex-col gap-[2px] items-start leading-[normal] relative shrink-0 text-[9px] w-full"
                data-node-id="I764:2242;713:369"
                data-name="Frame"
              >
                <p
                  className="font-display not-italic relative shrink-0 tracking-[1.5px] uppercase"
                  data-node-id="I764:2242;713:370"
                >
                  {text(
                    "I made an error during the order for the Compatibility report in STARYIELD. ",
                  )}
                </p>
                <p
                  className="font-serif font-normal relative shrink-0 tracking-[1px]"
                  data-node-id="I764:2242;713:371"
                >
                  {text("PREMIUM FUNCTIONALITY ")}
                </p>
                <p
                  className="font-serif font-semibold relative shrink-0 tracking-[1.5px] uppercase"
                  data-node-id="I764:2242;713:372"
                >
                  {text("INTEGRITY RE-AUDIT ")}
                </p>
              </div>
            </ContentLink>
            <ContentLink
              className="content-stretch flex flex-col gap-[8px] items-start overflow-clip pb-[32px] relative shrink-0 w-[495px]"
              data-node-id="764:2249"
              data-name="Instance / State=Collapsed / 04"
              label="↑ My STARYIELD subscription was activated without 3-days trial PREMIUM FUNCTIONALITY BILLING DISCREPANCY"
            >
              <p
                className="font-serif font-normal leading-[28px] relative shrink-0 text-[22px]"
                data-node-id="I764:2249;725:1130"
              >
                ↑
              </p>
              <div
                className="content-stretch flex flex-col gap-[2px] items-start leading-[normal] relative shrink-0 text-[9px] w-full"
                data-node-id="I764:2249;713:369"
                data-name="Frame"
              >
                <p
                  className="font-display not-italic relative shrink-0 tracking-[1.5px] uppercase"
                  data-node-id="I764:2249;713:370"
                >
                  {text(
                    "My STARYIELD subscription was activated without 3-days trial ",
                  )}
                </p>
                <p
                  className="font-serif font-normal relative shrink-0 tracking-[1px]"
                  data-node-id="I764:2249;713:371"
                >
                  {text("PREMIUM FUNCTIONALITY ")}
                </p>
                <p
                  className="font-serif font-semibold relative shrink-0 tracking-[1.5px] uppercase"
                  data-node-id="I764:2249;713:372"
                >
                  {text("BILLING DISCREPANCY ")}
                </p>
              </div>
            </ContentLink>
            <ContentLink
              className="content-stretch flex flex-col gap-[8px] items-start overflow-clip pb-[32px] relative shrink-0 w-[495px]"
              data-node-id="764:2256"
              data-name="Instance / State=Collapsed / 05"
              label="↑ I have won a subscription on STARYIELD in a SpinWheel but it is not visible in the subscriptions list. PREMIUM FUNCTIONALITY GAMIFICATION ANOMALY"
            >
              <p
                className="font-serif font-normal leading-[28px] relative shrink-0 text-[22px]"
                data-node-id="I764:2256;725:1130"
              >
                ↑
              </p>
              <div
                className="content-stretch flex flex-col gap-[2px] items-start leading-[normal] relative shrink-0 text-[9px] w-full"
                data-node-id="I764:2256;713:369"
                data-name="Frame"
              >
                <p
                  className="font-display not-italic relative shrink-0 tracking-[1.5px] uppercase"
                  data-node-id="I764:2256;713:370"
                >
                  {text(
                    "I have won a subscription on STARYIELD in a SpinWheel but it is not visible in the subscriptions list. ",
                  )}
                </p>
                <p
                  className="font-serif font-normal relative shrink-0 tracking-[1px]"
                  data-node-id="I764:2256;713:371"
                >
                  {text("PREMIUM FUNCTIONALITY ")}
                </p>
                <p
                  className="font-serif font-semibold relative shrink-0 tracking-[1.5px] uppercase"
                  data-node-id="I764:2256;713:372"
                >
                  {text("GAMIFICATION ANOMALY ")}
                </p>
              </div>
            </ContentLink>
            <ContentLink
              className="content-stretch flex flex-col gap-[8px] items-start overflow-clip pb-[32px] relative shrink-0 w-[495px]"
              data-node-id="764:2263"
              data-name="Instance / State=Collapsed / 06"
              label="↑ I have purchased a Premium subscription in STARYIELD but it is not visible in the subscriptions list. PREMIUM FUNCTIONALITY TRANSACTION SYNC"
            >
              <p
                className="font-serif font-normal leading-[28px] relative shrink-0 text-[22px]"
                data-node-id="I764:2263;725:1130"
              >
                ↑
              </p>
              <div
                className="content-stretch flex flex-col gap-[2px] items-start leading-[normal] relative shrink-0 text-[9px] w-full"
                data-node-id="I764:2263;713:369"
                data-name="Frame"
              >
                <p
                  className="font-display not-italic relative shrink-0 tracking-[1.5px] uppercase"
                  data-node-id="I764:2263;713:370"
                >
                  {text(
                    "I have purchased a Premium subscription in STARYIELD but it is not visible in the subscriptions list. ",
                  )}
                </p>
                <p
                  className="font-serif font-normal relative shrink-0 tracking-[1px]"
                  data-node-id="I764:2263;713:371"
                >
                  {text("PREMIUM FUNCTIONALITY ")}
                </p>
                <p
                  className="font-serif font-semibold relative shrink-0 tracking-[1.5px] uppercase"
                  data-node-id="I764:2263;713:372"
                >
                  {text("TRANSACTION SYNC ")}
                </p>
              </div>
            </ContentLink>
            <ContentLink
              className="content-stretch flex flex-col gap-[8px] items-start overflow-clip pb-[32px] relative shrink-0 w-[495px]"
              data-node-id="764:2270"
              data-name="Instance / State=Collapsed / 07"
              label="↑ How can I change STARYIELD subscription price or duration on Android? PREMIUM FUNCTIONALITY PLAY STORE CONTROL"
            >
              <p
                className="font-serif font-normal leading-[28px] relative shrink-0 text-[22px]"
                data-node-id="I764:2270;725:1130"
              >
                ↑
              </p>
              <div
                className="content-stretch flex flex-col gap-[2px] items-start leading-[normal] relative shrink-0 text-[9px] w-full"
                data-node-id="I764:2270;713:369"
                data-name="Frame"
              >
                <p
                  className="font-display not-italic relative shrink-0 tracking-[1.5px] uppercase"
                  data-node-id="I764:2270;713:370"
                >
                  {text(
                    "How can I change STARYIELD subscription price or duration on Android? ",
                  )}
                </p>
                <p
                  className="font-serif font-normal relative shrink-0 tracking-[1px]"
                  data-node-id="I764:2270;713:371"
                >
                  {text("PREMIUM FUNCTIONALITY ")}
                </p>
                <p
                  className="font-serif font-semibold relative shrink-0 tracking-[1.5px] uppercase"
                  data-node-id="I764:2270;713:372"
                >
                  {text("PLAY STORE CONTROL ")}
                </p>
              </div>
            </ContentLink>
            <ContentLink
              className="content-stretch flex flex-col gap-[8px] items-start overflow-clip pb-[32px] relative shrink-0 w-[495px]"
              data-node-id="764:2277"
              data-name="Instance / State=Collapsed / 08"
              label="↑ How can I change STARYIELD subscription duration or price on iPhone\iPad? PREMIUM FUNCTIONALITY APP STORE CONTROL"
            >
              <p
                className="font-serif font-normal leading-[28px] relative shrink-0 text-[22px]"
                data-node-id="I764:2277;725:1130"
              >
                ↑
              </p>
              <div
                className="content-stretch flex flex-col gap-[2px] items-start leading-[normal] relative shrink-0 text-[9px] w-full"
                data-node-id="I764:2277;713:369"
                data-name="Frame"
              >
                <p
                  className="font-display not-italic relative shrink-0 tracking-[1.5px] uppercase"
                  data-node-id="I764:2277;713:370"
                >
                  {text(
                    "How can I change STARYIELD subscription duration or price on iPhone\\iPad? ",
                  )}
                </p>
                <p
                  className="font-serif font-normal relative shrink-0 tracking-[1px]"
                  data-node-id="I764:2277;713:371"
                >
                  {text("PREMIUM FUNCTIONALITY ")}
                </p>
                <p
                  className="font-serif font-semibold relative shrink-0 tracking-[1.5px] uppercase"
                  data-node-id="I764:2277;713:372"
                >
                  {text("APP STORE CONTROL ")}
                </p>
              </div>
            </ContentLink>
            <ContentLink
              className="content-stretch flex flex-col gap-[8px] items-start overflow-clip pb-[32px] relative shrink-0 w-[495px]"
              data-node-id="764:2284"
              data-name="Instance / State=Collapsed / 09"
              label="↑ I want one more report PREMIUM FUNCTIONALITY CELESTIAL ACQUISITIONS"
            >
              <p
                className="font-serif font-normal leading-[28px] relative shrink-0 text-[22px]"
                data-node-id="I764:2284;725:1130"
              >
                ↑
              </p>
              <div
                className="content-stretch flex flex-col gap-[2px] items-start leading-[normal] relative shrink-0 text-[9px] w-full"
                data-node-id="I764:2284;713:369"
                data-name="Frame"
              >
                <p
                  className="font-display not-italic relative shrink-0 tracking-[1.5px] uppercase"
                  data-node-id="I764:2284;713:370"
                >
                  {text("I want one more report ")}
                </p>
                <p
                  className="font-serif font-normal relative shrink-0 tracking-[1px]"
                  data-node-id="I764:2284;713:371"
                >
                  {text("PREMIUM FUNCTIONALITY ")}
                </p>
                <p
                  className="font-serif font-semibold relative shrink-0 tracking-[1.5px] uppercase"
                  data-node-id="I764:2284;713:372"
                >
                  {text("CELESTIAL ACQUISITIONS ")}
                </p>
              </div>
            </ContentLink>
            <ContentLink
              className="content-stretch flex flex-col gap-[8px] items-start overflow-clip pb-[32px] relative shrink-0 w-[495px]"
              data-node-id="764:2291"
              data-name="Instance / State=Collapsed / 10"
              label="↑ I have not received the Compatibility report from the STARYIELD app. PREMIUM FUNCTIONALITY DELIVERY SYNCHRONIZATION"
            >
              <p
                className="font-serif font-normal leading-[28px] relative shrink-0 text-[22px]"
                data-node-id="I764:2291;725:1130"
              >
                ↑
              </p>
              <div
                className="content-stretch flex flex-col gap-[2px] items-start leading-[normal] relative shrink-0 text-[9px] w-full"
                data-node-id="I764:2291;713:369"
                data-name="Frame"
              >
                <p
                  className="font-display not-italic relative shrink-0 tracking-[1.5px] uppercase"
                  data-node-id="I764:2291;713:370"
                >
                  {text(
                    "I have not received the Compatibility report from the STARYIELD app. ",
                  )}
                </p>
                <p
                  className="font-serif font-normal relative shrink-0 tracking-[1px]"
                  data-node-id="I764:2291;713:371"
                >
                  {text("PREMIUM FUNCTIONALITY ")}
                </p>
                <p
                  className="font-serif font-semibold relative shrink-0 tracking-[1.5px] uppercase"
                  data-node-id="I764:2291;713:372"
                >
                  {text("DELIVERY SYNCHRONIZATION ")}
                </p>
              </div>
            </ContentLink>
            <ContentLink
              className="content-stretch flex flex-col gap-[8px] items-start overflow-clip pb-[32px] relative shrink-0 w-[495px]"
              data-node-id="764:2298"
              data-name="Instance / State=Collapsed / 11"
              label="↑ Can not open the Compatibility report! PREMIUM FUNCTIONALITY FORMAT VERIFICATION"
            >
              <p
                className="font-serif font-normal leading-[28px] relative shrink-0 text-[22px]"
                data-node-id="I764:2298;725:1130"
              >
                ↑
              </p>
              <div
                className="content-stretch flex flex-col gap-[2px] items-start leading-[normal] relative shrink-0 text-[9px] w-full"
                data-node-id="I764:2298;713:369"
                data-name="Frame"
              >
                <p
                  className="font-display not-italic relative shrink-0 tracking-[1.5px] uppercase"
                  data-node-id="I764:2298;713:370"
                >
                  {text("Can not open the Compatibility report! ")}
                </p>
                <p
                  className="font-serif font-normal relative shrink-0 tracking-[1px]"
                  data-node-id="I764:2298;713:371"
                >
                  {text("PREMIUM FUNCTIONALITY ")}
                </p>
                <p
                  className="font-serif font-semibold relative shrink-0 tracking-[1.5px] uppercase"
                  data-node-id="I764:2298;713:372"
                >
                  {text("FORMAT VERIFICATION ")}
                </p>
              </div>
            </ContentLink>
            <ContentLink
              className="content-stretch flex flex-col gap-[8px] items-start overflow-clip pb-[32px] relative shrink-0 w-[495px]"
              data-node-id="764:2305"
              data-name="Instance / State=Collapsed / 12"
              label="↑ How can I restore my STARYIELD subscription on iPhone\iPad? PREMIUM FUNCTIONALITY APPLE RESTORATION"
            >
              <p
                className="font-serif font-normal leading-[28px] relative shrink-0 text-[22px]"
                data-node-id="I764:2305;725:1130"
              >
                ↑
              </p>
              <div
                className="content-stretch flex flex-col gap-[2px] items-start leading-[normal] relative shrink-0 text-[9px] w-full"
                data-node-id="I764:2305;713:369"
                data-name="Frame"
              >
                <p
                  className="font-display not-italic relative shrink-0 tracking-[1.5px] uppercase"
                  data-node-id="I764:2305;713:370"
                >
                  {text(
                    "How can I restore my STARYIELD subscription on iPhone\\iPad? ",
                  )}
                </p>
                <p
                  className="font-serif font-normal relative shrink-0 tracking-[1px]"
                  data-node-id="I764:2305;713:371"
                >
                  {text("PREMIUM FUNCTIONALITY ")}
                </p>
                <p
                  className="font-serif font-semibold relative shrink-0 tracking-[1.5px] uppercase"
                  data-node-id="I764:2305;713:372"
                >
                  {text("APPLE RESTORATION ")}
                </p>
              </div>
            </ContentLink>
            <ContentLink
              className="content-stretch flex flex-col gap-[8px] items-start overflow-clip pb-[32px] relative shrink-0 w-[495px]"
              data-node-id="764:2312"
              data-name="Instance / State=Collapsed / 13"
              label="↑ How can I restore my STARYIELD subscription on Android? PREMIUM FUNCTIONALITY GOOGLE RESTORATION"
            >
              <p
                className="font-serif font-normal leading-[28px] relative shrink-0 text-[22px]"
                data-node-id="I764:2312;725:1130"
              >
                ↑
              </p>
              <div
                className="content-stretch flex flex-col gap-[2px] items-start leading-[normal] relative shrink-0 text-[9px] w-full"
                data-node-id="I764:2312;713:369"
                data-name="Frame"
              >
                <p
                  className="font-display not-italic relative shrink-0 tracking-[1.5px] uppercase"
                  data-node-id="I764:2312;713:370"
                >
                  {text(
                    "How can I restore my STARYIELD subscription on Android? ",
                  )}
                </p>
                <p
                  className="font-serif font-normal relative shrink-0 tracking-[1px]"
                  data-node-id="I764:2312;713:371"
                >
                  {text("PREMIUM FUNCTIONALITY ")}
                </p>
                <p
                  className="font-serif font-semibold relative shrink-0 tracking-[1.5px] uppercase"
                  data-node-id="I764:2312;713:372"
                >
                  {text("GOOGLE RESTORATION ")}
                </p>
              </div>
            </ContentLink>
            <ContentLink
              className="content-stretch flex flex-col gap-[8px] items-start overflow-clip pb-[32px] relative shrink-0 w-[495px]"
              data-node-id="764:2319"
              data-name="Instance / State=Collapsed / 14"
              label="↑ How can I cancel my STARYIELD subscription on iPhone\iPad? PREMIUM FUNCTIONALITY APPLE DISMISSAL"
            >
              <p
                className="font-serif font-normal leading-[28px] relative shrink-0 text-[22px]"
                data-node-id="I764:2319;725:1130"
              >
                ↑
              </p>
              <div
                className="content-stretch flex flex-col gap-[2px] items-start leading-[normal] relative shrink-0 text-[9px] w-full"
                data-node-id="I764:2319;713:369"
                data-name="Frame"
              >
                <p
                  className="font-display not-italic relative shrink-0 tracking-[1.5px] uppercase"
                  data-node-id="I764:2319;713:370"
                >
                  {text(
                    "How can I cancel my STARYIELD subscription on iPhone\\iPad? ",
                  )}
                </p>
                <p
                  className="font-serif font-normal relative shrink-0 tracking-[1px]"
                  data-node-id="I764:2319;713:371"
                >
                  {text("PREMIUM FUNCTIONALITY ")}
                </p>
                <p
                  className="font-serif font-semibold relative shrink-0 tracking-[1.5px] uppercase"
                  data-node-id="I764:2319;713:372"
                >
                  {text("APPLE DISMISSAL ")}
                </p>
              </div>
            </ContentLink>
            <ContentLink
              className="content-stretch flex flex-col gap-[8px] items-start overflow-clip pb-[32px] relative shrink-0 w-[495px]"
              data-node-id="764:2326"
              data-name="Instance / State=Collapsed / 15"
              label="↑ How can I cancel my STARYIELD subscription on Android? PREMIUM FUNCTIONALITY GOOGLE DISMISSAL"
            >
              <p
                className="font-serif font-normal leading-[28px] relative shrink-0 text-[22px]"
                data-node-id="I764:2326;725:1130"
              >
                ↑
              </p>
              <div
                className="content-stretch flex flex-col gap-[2px] items-start leading-[normal] relative shrink-0 text-[9px] w-full"
                data-node-id="I764:2326;713:369"
                data-name="Frame"
              >
                <p
                  className="font-display not-italic relative shrink-0 tracking-[1.5px] uppercase"
                  data-node-id="I764:2326;713:370"
                >
                  {text(
                    "How can I cancel my STARYIELD subscription on Android? ",
                  )}
                </p>
                <p
                  className="font-serif font-normal relative shrink-0 tracking-[1px]"
                  data-node-id="I764:2326;713:371"
                >
                  {text("PREMIUM FUNCTIONALITY ")}
                </p>
                <p
                  className="font-serif font-semibold relative shrink-0 tracking-[1.5px] uppercase"
                  data-node-id="I764:2326;713:372"
                >
                  {text("GOOGLE DISMISSAL ")}
                </p>
              </div>
            </ContentLink>
          </section>
        </section>
        <div
          className="h-0 relative shrink-0 w-full"
          data-node-id="764:2333"
          data-name="Divider / Horizontal / 05"
        >
          <div className="absolute inset-[-1px_0_0_0]">
            <img
              alt=""
              className="block max-w-none size-full"
              src={imgDividerHorizontal}
            />
          </div>
        </div>
        <section
          className="content-stretch flex gap-[40px] items-start relative shrink-0 w-full figma-section"
          data-node-id="764:2334"
          data-name="Section / FAQ Block / 06"
        >
          <section
            className="content-stretch flex flex-col items-start relative shrink-0 w-[455px] figma-section"
            data-node-id="764:2335"
            data-name="Section / Left Column"
          >
            <div
              className="content-stretch flex gap-[12px] items-center pb-[16px] relative shrink-0"
              data-node-id="764:2336"
              data-name="Header / Category"
            >
              <div
                className="relative shrink-0 size-[34.07px]"
                data-node-id="764:2337"
                data-name="Instance / Star"
              >
                <img
                  alt=""
                  className="absolute block inset-0 max-w-none size-full"
                  height="34.07"
                  src={imgInstanceStar1}
                  width="34.07"
                />
              </div>
              <p
                className="[word-break:break-word] font-display leading-[normal] not-italic relative shrink-0 text-[20px] text-black tracking-[0.6px] whitespace-nowrap"
                data-node-id="764:2339"
              >
                {text("STARYIELD Astrologers ")}
              </p>
            </div>
            <p
              className="[word-break:break-word] font-serif font-normal leading-[22px] min-w-full relative shrink-0 text-[14px] text-black w-[min-content]"
              data-node-id="764:2340"
            >
              {text(
                "Refund variables, chat histories, technical issues, and how communication costs are calculated securely. ",
              )}
            </p>
          </section>
          <section
            className="[word-break:break-word] content-stretch cursor-pointer flex flex-col gap-[16px] items-start relative shrink-0 text-black text-left w-[705px] whitespace-nowrap figma-section"
            data-node-id="764:2341"
            data-name="Section / Right Column"
          >
            <ContentLink
              className="content-stretch flex flex-col gap-[8px] items-start pb-[32px] pt-[28px] relative shrink-0 w-[495px]"
              data-node-id="764:2342"
              data-name="Instance / State=Collapsed"
              label="☉ How can I receive the refund of credits for chatting with astrologers in STARYIELD? ASTROLOGICAL INTEGRITY REFUND PROTOCOLS"
            >
              <p
                className="font-serif font-normal leading-[28px] relative shrink-0 text-[22px]"
                data-node-id="I764:2342;713:386"
              >
                ☉
              </p>
              <div
                className="content-stretch flex flex-col gap-[2px] items-start leading-[normal] relative shrink-0 text-[9px] w-full"
                data-node-id="I764:2342;713:387"
                data-name="Frame"
              >
                <p
                  className="font-display not-italic relative shrink-0 tracking-[1.5px] uppercase"
                  data-node-id="I764:2342;713:388"
                >
                  {text(
                    "How can I receive the refund of credits for chatting with astrologers in STARYIELD? ",
                  )}
                </p>
                <p
                  className="font-serif font-normal relative shrink-0 tracking-[1px]"
                  data-node-id="I764:2342;713:389"
                >
                  {text("ASTROLOGICAL INTEGRITY ")}
                </p>
                <p
                  className="font-serif font-semibold relative shrink-0 tracking-[1.5px] uppercase"
                  data-node-id="I764:2342;713:390"
                >
                  {text("REFUND PROTOCOLS ")}
                </p>
              </div>
            </ContentLink>
            <ContentLink
              className="content-stretch flex flex-col gap-[8px] items-start pb-[32px] pt-[28px] relative shrink-0 w-[495px]"
              data-node-id="764:2349"
              data-name="Instance / State=Collapsed / 02"
              label="☉ My chats' history has disappeared after the reinstallation of STARYIELD app. ASTROLOGICAL INTEGRITY TECHNICAL CACHE"
            >
              <p
                className="font-serif font-normal leading-[28px] relative shrink-0 text-[22px]"
                data-node-id="I764:2349;713:386"
              >
                ☉
              </p>
              <div
                className="content-stretch flex flex-col gap-[2px] items-start leading-[normal] relative shrink-0 text-[9px] w-full"
                data-node-id="I764:2349;713:387"
                data-name="Frame"
              >
                <p
                  className="font-display not-italic relative shrink-0 tracking-[1.5px] uppercase"
                  data-node-id="I764:2349;713:388"
                >
                  {text(
                    `My chats' history has disappeared after the reinstallation of STARYIELD app.`,
                  )}
                </p>
                <p
                  className="font-serif font-normal relative shrink-0 tracking-[1px]"
                  data-node-id="I764:2349;713:389"
                >
                  {text("ASTROLOGICAL INTEGRITY ")}
                </p>
                <p
                  className="font-serif font-semibold relative shrink-0 tracking-[1.5px] uppercase"
                  data-node-id="I764:2349;713:390"
                >
                  {text("TECHNICAL CACHE ")}
                </p>
              </div>
            </ContentLink>
            <ContentLink
              className="content-stretch flex flex-col gap-[8px] items-start pb-[32px] pt-[28px] relative shrink-0 w-[495px]"
              data-node-id="764:2356"
              data-name="Instance / State=Collapsed / 03"
              label="☉ How can I choose an astrologer? ASTROLOGICAL INTEGRITY MATCHING MATRIX"
            >
              <p
                className="font-serif font-normal leading-[28px] relative shrink-0 text-[22px]"
                data-node-id="I764:2356;713:386"
              >
                ☉
              </p>
              <div
                className="content-stretch flex flex-col gap-[2px] items-start leading-[normal] relative shrink-0 text-[9px] w-full"
                data-node-id="I764:2356;713:387"
                data-name="Frame"
              >
                <p
                  className="font-display not-italic relative shrink-0 tracking-[1.5px] uppercase"
                  data-node-id="I764:2356;713:388"
                >
                  {text("How can I choose an astrologer? ")}
                </p>
                <p
                  className="font-serif font-normal relative shrink-0 tracking-[1px]"
                  data-node-id="I764:2356;713:389"
                >
                  {text("ASTROLOGICAL INTEGRITY ")}
                </p>
                <p
                  className="font-serif font-semibold relative shrink-0 tracking-[1.5px] uppercase"
                  data-node-id="I764:2356;713:390"
                >
                  {text("MATCHING MATRIX ")}
                </p>
              </div>
            </ContentLink>
            <ContentLink
              className="content-stretch flex flex-col gap-[8px] items-start pb-[32px] pt-[28px] relative shrink-0 w-[495px]"
              data-node-id="764:2363"
              data-name="Instance / State=Collapsed / 04"
              label="☉ My minutes in STARYIELD are running out very fast. ASTROLOGICAL INTEGRITY CHRONOLOGICAL SPEEDS"
            >
              <p
                className="font-serif font-normal leading-[28px] relative shrink-0 text-[22px]"
                data-node-id="I764:2363;713:386"
              >
                ☉
              </p>
              <div
                className="content-stretch flex flex-col gap-[2px] items-start leading-[normal] relative shrink-0 text-[9px] w-full"
                data-node-id="I764:2363;713:387"
                data-name="Frame"
              >
                <p
                  className="font-display not-italic relative shrink-0 tracking-[1.5px] uppercase"
                  data-node-id="I764:2363;713:388"
                >
                  {text("My minutes in STARYIELD are running out very fast. ")}
                </p>
                <p
                  className="font-serif font-normal relative shrink-0 tracking-[1px]"
                  data-node-id="I764:2363;713:389"
                >
                  {text("ASTROLOGICAL INTEGRITY ")}
                </p>
                <p
                  className="font-serif font-semibold relative shrink-0 tracking-[1.5px] uppercase"
                  data-node-id="I764:2363;713:390"
                >
                  {text("CHRONOLOGICAL SPEEDS ")}
                </p>
              </div>
            </ContentLink>
            <ContentLink
              className="content-stretch flex flex-col gap-[8px] items-start pb-[32px] pt-[28px] relative shrink-0 w-[495px]"
              data-node-id="764:2370"
              data-name="Instance / State=Collapsed / 05"
              label="☉ My credits on STARYIELD disappeared. (Tech issue) ASTROLOGICAL INTEGRITY BALANCE RETRIEVAL"
            >
              <p
                className="font-serif font-normal leading-[28px] relative shrink-0 text-[22px]"
                data-node-id="I764:2370;713:386"
              >
                ☉
              </p>
              <div
                className="content-stretch flex flex-col gap-[2px] items-start leading-[normal] relative shrink-0 text-[9px] w-full"
                data-node-id="I764:2370;713:387"
                data-name="Frame"
              >
                <p
                  className="font-display not-italic relative shrink-0 tracking-[1.5px] uppercase"
                  data-node-id="I764:2370;713:388"
                >
                  {text("My credits on STARYIELD disappeared. (Tech issue) ")}
                </p>
                <p
                  className="font-serif font-normal relative shrink-0 tracking-[1px]"
                  data-node-id="I764:2370;713:389"
                >
                  {text("ASTROLOGICAL INTEGRITY ")}
                </p>
                <p
                  className="font-serif font-semibold relative shrink-0 tracking-[1.5px] uppercase"
                  data-node-id="I764:2370;713:390"
                >
                  {text("BALANCE RETRIEVAL ")}
                </p>
              </div>
            </ContentLink>
            <ContentLink
              className="content-stretch flex flex-col gap-[8px] items-start pb-[32px] pt-[28px] relative shrink-0 w-[495px]"
              data-node-id="764:2377"
              data-name="Instance / State=Collapsed / 06"
              label="☉ How much does it cost to communicate with astrologers in STARYIELD? ASTROLOGICAL INTEGRITY COMPENSATION MODELS"
            >
              <p
                className="font-serif font-normal leading-[28px] relative shrink-0 text-[22px]"
                data-node-id="I764:2377;713:386"
              >
                ☉
              </p>
              <div
                className="content-stretch flex flex-col gap-[2px] items-start leading-[normal] relative shrink-0 text-[9px] w-full"
                data-node-id="I764:2377;713:387"
                data-name="Frame"
              >
                <p
                  className="font-display not-italic relative shrink-0 tracking-[1.5px] uppercase"
                  data-node-id="I764:2377;713:388"
                >
                  {text(
                    "How much does it cost to communicate with astrologers in STARYIELD? ",
                  )}
                </p>
                <p
                  className="font-serif font-normal relative shrink-0 tracking-[1px]"
                  data-node-id="I764:2377;713:389"
                >
                  {text("ASTROLOGICAL INTEGRITY ")}
                </p>
                <p
                  className="font-serif font-semibold relative shrink-0 tracking-[1.5px] uppercase"
                  data-node-id="I764:2377;713:390"
                >
                  {text("COMPENSATION MODELS ")}
                </p>
              </div>
            </ContentLink>
            <ContentLink
              className="content-stretch flex flex-col gap-[8px] items-start pb-[32px] pt-[28px] relative shrink-0 w-[495px]"
              data-node-id="764:2384"
              data-name="Instance / State=Collapsed / 07"
              label="☉ How can I start chatting with astrologers in STARYIELD? ASTROLOGICAL INTEGRITY INITIATION PATHWAY"
            >
              <p
                className="font-serif font-normal leading-[28px] relative shrink-0 text-[22px]"
                data-node-id="I764:2384;713:386"
              >
                ☉
              </p>
              <div
                className="content-stretch flex flex-col gap-[2px] items-start leading-[normal] relative shrink-0 text-[9px] w-full"
                data-node-id="I764:2384;713:387"
                data-name="Frame"
              >
                <p
                  className="font-display not-italic relative shrink-0 tracking-[1.5px] uppercase"
                  data-node-id="I764:2384;713:388"
                >
                  {text(
                    "How can I start chatting with astrologers in STARYIELD? ",
                  )}
                </p>
                <p
                  className="font-serif font-normal relative shrink-0 tracking-[1px]"
                  data-node-id="I764:2384;713:389"
                >
                  {text("ASTROLOGICAL INTEGRITY ")}
                </p>
                <p
                  className="font-serif font-semibold relative shrink-0 tracking-[1.5px] uppercase"
                  data-node-id="I764:2384;713:390"
                >
                  {text("INITIATION PATHWAY ")}
                </p>
              </div>
            </ContentLink>
            <ContentLink
              className="content-stretch flex flex-col gap-[8px] items-start pb-[32px] pt-[28px] relative shrink-0 w-[495px]"
              data-node-id="764:2391"
              data-name="Instance / State=Collapsed / 08"
              label="☉ I am not satisfied with the astrologer's service at STARYIELD ASTROLOGICAL INTEGRITY SERVICE ASSURANCE"
            >
              <p
                className="font-serif font-normal leading-[28px] relative shrink-0 text-[22px]"
                data-node-id="I764:2391;713:386"
              >
                ☉
              </p>
              <div
                className="content-stretch flex flex-col gap-[2px] items-start leading-[normal] relative shrink-0 text-[9px] w-full"
                data-node-id="I764:2391;713:387"
                data-name="Frame"
              >
                <p
                  className="font-display not-italic relative shrink-0 tracking-[1.5px] uppercase"
                  data-node-id="I764:2391;713:388"
                >
                  {text(
                    `I am not satisfied with the astrologer's service at STARYIELD`,
                  )}
                </p>
                <p
                  className="font-serif font-normal relative shrink-0 tracking-[1px]"
                  data-node-id="I764:2391;713:389"
                >
                  {text("ASTROLOGICAL INTEGRITY ")}
                </p>
                <p
                  className="font-serif font-semibold relative shrink-0 tracking-[1.5px] uppercase"
                  data-node-id="I764:2391;713:390"
                >
                  {text("SERVICE ASSURANCE ")}
                </p>
              </div>
            </ContentLink>
            <ContentLink
              className="content-stretch flex flex-col gap-[8px] items-start pb-[32px] pt-[28px] relative shrink-0 w-[495px]"
              data-node-id="764:2398"
              data-name="Instance / State=Collapsed / 09"
              label="☉ Astrologer has never answered my questions in STARYIELD. ASTROLOGICAL INTEGRITY DELIVERY COMPLIANCE"
            >
              <p
                className="font-serif font-normal leading-[28px] relative shrink-0 text-[22px]"
                data-node-id="I764:2398;713:386"
              >
                ☉
              </p>
              <div
                className="content-stretch flex flex-col gap-[2px] items-start leading-[normal] relative shrink-0 text-[9px] w-full"
                data-node-id="I764:2398;713:387"
                data-name="Frame"
              >
                <p
                  className="font-display not-italic relative shrink-0 tracking-[1.5px] uppercase"
                  data-node-id="I764:2398;713:388"
                >
                  {text(
                    "Astrologer has never answered my questions in STARYIELD. ",
                  )}
                </p>
                <p
                  className="font-serif font-normal relative shrink-0 tracking-[1px]"
                  data-node-id="I764:2398;713:389"
                >
                  {text("ASTROLOGICAL INTEGRITY ")}
                </p>
                <p
                  className="font-serif font-semibold relative shrink-0 tracking-[1.5px] uppercase"
                  data-node-id="I764:2398;713:390"
                >
                  {text("DELIVERY COMPLIANCE ")}
                </p>
              </div>
            </ContentLink>
          </section>
        </section>
        <div
          className="h-0 relative shrink-0 w-full"
          data-node-id="764:2405"
          data-name="Divider / Horizontal / 06"
        >
          <div className="absolute inset-[-1px_0_0_0]">
            <img
              alt=""
              className="block max-w-none size-full"
              src={imgDividerHorizontal}
            />
          </div>
        </div>
        <section
          className="content-stretch flex gap-[40px] items-start relative shrink-0 w-full figma-section"
          data-node-id="764:2406"
          data-name="Section / FAQ Block / 07"
        >
          <section
            className="content-stretch flex flex-col items-start relative shrink-0 w-[455px] figma-section"
            data-node-id="764:2407"
            data-name="Section / Left Column"
          >
            <div
              className="content-stretch flex gap-[12px] items-center pb-[16px] relative shrink-0"
              data-node-id="764:2408"
              data-name="Header / Category"
            >
              <div
                className="relative shrink-0 size-[34.07px]"
                data-node-id="764:2409"
                data-name="Instance / Star"
              >
                <img
                  alt=""
                  className="absolute block inset-0 max-w-none size-full"
                  height="34.07"
                  src={imgInstanceStar1}
                  width="34.07"
                />
              </div>
              <p
                className="[word-break:break-word] font-display leading-[normal] not-italic relative shrink-0 text-[20px] text-black tracking-[0.6px] whitespace-nowrap"
                data-node-id="764:2411"
              >
                {text("STARYIELD Billing ")}
              </p>
            </div>
            <p
              className="[word-break:break-word] font-serif font-normal leading-[22px] min-w-full relative shrink-0 text-[14px] text-black w-[min-content]"
              data-node-id="764:2412"
            >
              {text(
                "Transactions, balance updates, refunds, trial errors, and resolving app store technical purchase loops. ",
              )}
            </p>
          </section>
          <section
            className="[word-break:break-word] content-stretch flex flex-col gap-[16px] items-start relative shrink-0 text-black w-[705px] whitespace-nowrap figma-section"
            data-node-id="764:2413"
            data-name="Section / Right Column"
          >
            <div
              className="content-stretch flex flex-col gap-[8px] items-start pb-[32px] pt-[28px] relative shrink-0 w-[780px]"
              data-node-id="764:2414"
              data-name="Instance / State=Collapsed"
            >
              <p
                className="font-serif font-normal leading-[28px] relative shrink-0 text-[22px]"
                data-node-id="I764:2414;713:403"
              >
                ☽
              </p>
              <div
                className="content-stretch flex flex-col gap-[2px] items-start leading-[normal] relative shrink-0 text-[9px] w-full"
                data-node-id="I764:2414;713:404"
                data-name="Frame"
              >
                <p
                  className="font-display not-italic relative shrink-0 tracking-[1.5px] uppercase"
                  data-node-id="I764:2414;713:405"
                >
                  {text(
                    "My credits have disappeared from my balance after the reinstallation of the STARYIELD app. ",
                  )}
                </p>
                <p
                  className="font-serif font-normal relative shrink-0 tracking-[1px]"
                  data-node-id="I764:2414;713:406"
                >
                  {text("FINANCIAL INFRASTRUCTURE ")}
                </p>
                <p
                  className="font-serif font-semibold relative shrink-0 tracking-[1.5px] uppercase"
                  data-node-id="I764:2414;713:407"
                >
                  {text("METADATA RECOVERY ")}
                </p>
              </div>
            </div>
            <div
              className="content-stretch flex flex-col gap-[8px] items-start pb-[32px] pt-[28px] relative shrink-0 w-full"
              data-node-id="764:2421"
              data-name="Instance / State=Collapsed / 02"
            >
              <p
                className="font-serif font-normal leading-[28px] relative shrink-0 text-[22px]"
                data-node-id="I764:2421;713:403"
              >
                ☽
              </p>
              <div
                className="content-stretch flex flex-col gap-[2px] items-start leading-[normal] relative shrink-0 text-[9px] w-full"
                data-node-id="I764:2421;713:404"
                data-name="Frame"
              >
                <p
                  className="font-display not-italic relative shrink-0 tracking-[1.5px] uppercase"
                  data-node-id="I764:2421;713:405"
                >
                  {text(
                    "I have purchased credits in STARYIELD, but the balance was not topped up. ",
                  )}
                </p>
                <p
                  className="font-serif font-normal relative shrink-0 tracking-[1px]"
                  data-node-id="I764:2421;713:406"
                >
                  {text("FINANCIAL INFRASTRUCTURE ")}
                </p>
                <p
                  className="font-serif font-semibold relative shrink-0 tracking-[1.5px] uppercase"
                  data-node-id="I764:2421;713:407"
                >
                  {text("ACQUISITIONS SYNCHRONIZATION ")}
                </p>
              </div>
            </div>
            <div
              className="content-stretch flex flex-col gap-[8px] items-start pb-[32px] pt-[28px] relative shrink-0 w-full"
              data-node-id="764:2428"
              data-name="Instance / State=Collapsed / 03"
            >
              <p
                className="font-serif font-normal leading-[28px] relative shrink-0 text-[22px]"
                data-node-id="I764:2428;713:403"
              >
                ☽
              </p>
              <div
                className="content-stretch flex flex-col gap-[2px] items-start leading-[normal] relative shrink-0 text-[9px] w-full"
                data-node-id="I764:2428;713:404"
                data-name="Frame"
              >
                <p
                  className="font-display not-italic relative shrink-0 tracking-[1.5px] uppercase"
                  data-node-id="I764:2428;713:405"
                >
                  {text(
                    "How can I receive a refund or compensation for STARYIELD? ",
                  )}
                </p>
                <p
                  className="font-serif font-normal relative shrink-0 tracking-[1px]"
                  data-node-id="I764:2428;713:406"
                >
                  {text("FINANCIAL INFRASTRUCTURE ")}
                </p>
                <p
                  className="font-serif font-semibold relative shrink-0 tracking-[1.5px] uppercase"
                  data-node-id="I764:2428;713:407"
                >
                  {text("COMPENSATION SCHEMES ")}
                </p>
              </div>
            </div>
            <div
              className="content-stretch flex flex-col gap-[8px] items-start pb-[32px] pt-[28px] relative shrink-0 w-full"
              data-node-id="764:2435"
              data-name="Instance / State=Collapsed / 04"
            >
              <p
                className="font-serif font-normal leading-[28px] relative shrink-0 text-[22px]"
                data-node-id="I764:2435;713:403"
              >
                ☽
              </p>
              <div
                className="content-stretch flex flex-col gap-[2px] items-start leading-[normal] relative shrink-0 text-[9px] w-full"
                data-node-id="I764:2435;713:404"
                data-name="Frame"
              >
                <p
                  className="font-display not-italic relative shrink-0 tracking-[1.5px] uppercase"
                  data-node-id="I764:2435;713:405"
                >
                  {text("I have been charged instead of 3-days trial. ")}
                </p>
                <p
                  className="font-serif font-normal relative shrink-0 tracking-[1px]"
                  data-node-id="I764:2435;713:406"
                >
                  {text("FINANCIAL INFRASTRUCTURE ")}
                </p>
                <p
                  className="font-serif font-semibold relative shrink-0 tracking-[1.5px] uppercase"
                  data-node-id="I764:2435;713:407"
                >
                  {text("INTEGRITY ERROR AUDIT ")}
                </p>
              </div>
            </div>
            <div
              className="content-stretch flex flex-col gap-[8px] items-start pb-[32px] pt-[28px] relative shrink-0 w-full"
              data-node-id="764:2442"
              data-name="Instance / State=Collapsed / 05"
            >
              <p
                className="font-serif font-normal leading-[28px] relative shrink-0 text-[22px]"
                data-node-id="I764:2442;713:403"
              >
                ☽
              </p>
              <div
                className="content-stretch flex flex-col gap-[2px] items-start leading-[normal] relative shrink-0 text-[9px] w-full"
                data-node-id="I764:2442;713:404"
                data-name="Frame"
              >
                <p
                  className="font-display not-italic relative shrink-0 tracking-[1.5px] uppercase"
                  data-node-id="I764:2442;713:405"
                >
                  {text(
                    `Why am I getting a 'You already own this item' error when buying credits?`,
                  )}
                </p>
                <p
                  className="font-serif font-normal relative shrink-0 tracking-[1px]"
                  data-node-id="I764:2442;713:406"
                >
                  {text("FINANCIAL INFRASTRUCTURE ")}
                </p>
                <p
                  className="font-serif font-semibold relative shrink-0 tracking-[1.5px] uppercase"
                  data-node-id="I764:2442;713:407"
                >
                  {text("BILLING LOOP EXCLUSIONS ")}
                </p>
              </div>
            </div>
          </section>
        </section>
      </div>
    </div>
  );
}

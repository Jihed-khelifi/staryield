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
                `If you need help with your account, credits or readings, please contact us `,
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
                "How to start private chats with our advisors, and what to do if something goes wrong. ",
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
              label="☉ I didn’t get a response from an astrologer/I was not satisfied with the reading I got. CONSULTATION PROTOCOLS YOUR READING"
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
                  {text("YOUR READING ")}
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
              label="☉ I am not satisfied with the astrologer's service at STARYIELD ASTROLOGICAL INTEGRITY YOUR READING"
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
                  {text("YOUR READING ")}
                </p>
              </div>
            </ContentLink>
            <ContentLink
              className="content-stretch flex flex-col gap-[8px] items-start pb-[32px] pt-[28px] relative shrink-0 w-[495px]"
              data-node-id="764:2398"
              data-name="Instance / State=Collapsed / 09"
              label="☉ Astrologer has never answered my questions in STARYIELD. ASTROLOGICAL INTEGRITY RESPONSES"
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
                  {text("RESPONSES ")}
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
                "Purchases, balance updates, refunds and receipts. ",
              )}
            </p>
          </section>
          <section
            className="[word-break:break-word] content-stretch flex flex-col gap-[16px] items-start relative shrink-0 text-black w-[705px] whitespace-nowrap figma-section"
            data-node-id="764:2413"
            data-name="Section / Right Column"
          >
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
                  {text("PAYMENTS ")}
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
                  {text("PAYMENTS ")}
                </p>
                <p
                  className="font-serif font-semibold relative shrink-0 tracking-[1.5px] uppercase"
                  data-node-id="I764:2428;713:407"
                >
                  {text("COMPENSATION SCHEMES ")}
                </p>
              </div>
            </div>
          </section>
        </section>
        <div
          className="h-0 relative shrink-0 w-full"
          data-name="Divider / Horizontal / 07"
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
          data-name="Section / FAQ Block / 08"
          id="live-readings"
        >
          <section
            className="content-stretch flex flex-col items-start relative shrink-0 w-[455px] figma-section"
            data-name="Section / Left Column"
          >
            <div
              className="content-stretch flex gap-[12px] items-center pb-[16px] relative shrink-0"
              data-name="Header / Category"
            >
              <div
                className="relative shrink-0 size-[34.07px]"
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
              <h2 className="[word-break:break-word] font-display leading-[normal] not-italic relative shrink-0 text-[20px] text-black tracking-[0.6px]">
                {text("Live readings ")}
              </h2>
            </div>
            <p className="[word-break:break-word] font-serif font-normal leading-[22px] min-w-full relative shrink-0 text-[14px] text-black w-[min-content]">
              {text(
                "Messages, photos, ratings and notifications during and after your live readings.",
              )}
            </p>
          </section>
          <section
            className="[word-break:break-word] content-stretch flex flex-col gap-[16px] items-start relative shrink-0 text-black text-left w-[705px] figma-section"
            data-name="Section / Right Column"
          >
            <details
              className="faq-item content-stretch flex flex-col gap-[8px] items-start overflow-clip pb-[32px] relative shrink-0 w-[495px]"
              data-name="Instance / Live readings / 01"
            >
              <summary>
                <p className="font-serif font-normal leading-[28px] relative shrink-0 text-[22px]">
                  ↑
                </p>
                <div
                  className="content-stretch flex flex-col gap-[2px] items-start leading-[normal] relative shrink-0 text-[9px] w-full"
                  data-name="Frame"
                >
                  <p className="font-display not-italic relative shrink-0 tracking-[1.5px] uppercase whitespace-normal">
                    {text("How do I rate my advisor?")}
                  </p>
                </div>
              </summary>
              <p className="mt-4 font-serif text-sm leading-[22px] tracking-normal normal-case whitespace-normal">
                {text(
                  "When your live reading ends, the chat summary invites you to rate your advisor from one to five stars and add an optional comment. You can also open the reminder from your notifications. Each reading can be rated once, within 7 days of its end.",
                )}
              </p>
            </details>
            <details
              className="faq-item content-stretch flex flex-col gap-[8px] items-start overflow-clip pb-[32px] relative shrink-0 w-[495px]"
              data-name="Instance / Live readings / 02"
            >
              <summary>
                <p className="font-serif font-normal leading-[28px] relative shrink-0 text-[22px]">
                  ↑
                </p>
                <div
                  className="content-stretch flex flex-col gap-[2px] items-start leading-[normal] relative shrink-0 text-[9px] w-full"
                  data-name="Frame"
                >
                  <p className="font-display not-italic relative shrink-0 tracking-[1.5px] uppercase whitespace-normal">
                    {text("Can I share photos during a reading?")}
                  </p>
                </div>
              </summary>
              <p className="mt-4 font-serif text-sm leading-[22px] tracking-normal normal-case whitespace-normal">
                {text(
                  "Yes. Use the image button next to the message box, paste an image or drag it into the chat. Only images (PNG, JPEG, WebP or GIF) up to 8 MB can be shared, and location data is removed automatically.",
                )}
              </p>
            </details>
            <details
              className="faq-item content-stretch flex flex-col gap-[8px] items-start overflow-clip pb-[32px] relative shrink-0 w-[495px]"
              data-name="Instance / Live readings / 03"
            >
              <summary>
                <p className="font-serif font-normal leading-[28px] relative shrink-0 text-[22px]">
                  ↑
                </p>
                <div
                  className="content-stretch flex flex-col gap-[2px] items-start leading-[normal] relative shrink-0 text-[9px] w-full"
                  data-name="Frame"
                >
                  <p className="font-display not-italic relative shrink-0 tracking-[1.5px] uppercase whitespace-normal">
                    {text("Who can see the photos I share?")}
                  </p>
                </div>
              </summary>
              <p className="mt-4 font-serif text-sm leading-[22px] tracking-normal normal-case whitespace-normal">
                {text(
                  "Only you and the advisor in that conversation. Images are stored privately and are never published, and Staryield staff cannot open them through our tools.",
                )}
              </p>
            </details>
            <details
              className="faq-item content-stretch flex flex-col gap-[8px] items-start overflow-clip pb-[32px] relative shrink-0 w-[495px]"
              data-name="Instance / Live readings / 04"
            >
              <summary>
                <p className="font-serif font-normal leading-[28px] relative shrink-0 text-[22px]">
                  ↑
                </p>
                <div
                  className="content-stretch flex flex-col gap-[2px] items-start leading-[normal] relative shrink-0 text-[9px] w-full"
                  data-name="Frame"
                >
                  <p className="font-display not-italic relative shrink-0 tracking-[1.5px] uppercase whitespace-normal">
                    {text("What does my advisor know about me?")}
                  </p>
                </div>
              </summary>
              <p className="mt-4 font-serif text-sm leading-[22px] tracking-normal normal-case whitespace-normal">
                {text(
                  "Your first name or nickname, profile picture, country, age, your astrological signs and daily horoscope, whether you are online and how many readings you have had together. Advisors never see your email address, balance or purchases.",
                )}
              </p>
            </details>
            <details
              className="faq-item content-stretch flex flex-col gap-[8px] items-start overflow-clip pb-[32px] relative shrink-0 w-[495px]"
              data-name="Instance / Live readings / 05"
            >
              <summary>
                <p className="font-serif font-normal leading-[28px] relative shrink-0 text-[22px]">
                  ↑
                </p>
                <div
                  className="content-stretch flex flex-col gap-[2px] items-start leading-[normal] relative shrink-0 text-[9px] w-full"
                  data-name="Frame"
                >
                  <p className="font-display not-italic relative shrink-0 tracking-[1.5px] uppercase whitespace-normal">
                    {text("How do notifications work?")}
                  </p>
                </div>
              </summary>
              <p className="mt-4 font-serif text-sm leading-[22px] tracking-normal normal-case whitespace-normal">
                {text(
                  "The bell at the top of the page shows chat requests, reading summaries, order updates and announcements in your language. Select a notification to open it, or mark them all as read.",
                )}
              </p>
            </details>
          </section>
        </section>
      </div>
    </div>
  );
}

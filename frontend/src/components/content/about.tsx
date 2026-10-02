"use client";
import { useI18n } from "@/i18n/i18n-provider";
/* Figma 478:1638 — reusable responsive content regions. */
/* eslint-disable @next/next/no-img-element */
import { ContentLink } from "@/components/content/content-link";
const assetPathPrefix = "https://assets.staryield.net/assets/figma";
const imgMediaAbout = `${assetPathPrefix}/fb9ee.jpg`;
const imgInstanceStar = `${assetPathPrefix}/7d7cc.png`;
const imgInstanceStar1 = `${assetPathPrefix}/b86b4.png`;
const imgInstanceStar2 = `${assetPathPrefix}/08e18.png`;
const imgInstanceStar3 = `${assetPathPrefix}/ef883.png`;
const imgDividerHorizontal = `${assetPathPrefix}/a0e30.svg`;
const imgDividerHorizontal1 = `${assetPathPrefix}/96fb1.svg`;
export default function ScreenDesktopAbout() {
  const { text } = useI18n();
  return (
    <div
      className="figma-content flex w-full flex-col items-center bg-cream"
      data-node-id="478:1638"
      data-name="Screen / Desktop / About"
    >
      <section
        className="bg-[#e0d5c3] content-stretch flex flex-col h-[384px] items-center justify-between overflow-clip pb-[120px] pt-[81px] px-6 md:px-10 lg:px-20 relative shrink-0 w-full figma-section"
        data-node-id="478:1668"
        data-name="Section / Hero"
      >
        <div
          className="content-stretch flex gap-[8px] items-center relative shrink-0"
          data-node-id="478:1670"
          data-name="Control / ESTABLISHED IN WISDOM"
        >
          <div
            className="relative shrink-0 size-[18px]"
            data-node-id="478:1671"
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
            className="[word-break:break-word] font-display leading-[normal] not-italic relative shrink-0 text-[14px] text-black tracking-[0.42px] whitespace-nowrap"
            data-node-id="478:1673"
          >
            {text("ESTABLISHED IN WISDOM ")}
          </p>
          <div
            className="relative shrink-0 size-[18px]"
            data-node-id="478:1674"
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
          className="[word-break:break-word] font-serif font-light leading-[48px] relative shrink-0 text-[34px] text-black text-center w-[1000px]"
          data-node-id="478:1676"
        >
          {text(
            "Staryield is your space for spiritual self-discovery, offering personalized tools and guidance to help you feel seen, connected, and confident along the way. ",
          )}
        </h1>
      </section>
      <section
        className="bg-gradient-to-b border-[rgba(194,154,61,0.15)] border-b border-solid border-t content-stretch flex flex-col from-[var(--cream)] gap-[56px] items-center px-6 md:px-10 lg:px-20 py-14 lg:py-[100px] relative shrink-0 to-[81.25%] to-[var(--warm-gray)] w-full figma-section"
        data-node-id="478:1677"
        data-name="Section / Mission & Vision"
      >
        <div
          className="content-stretch flex gap-[16px] items-center relative shrink-0"
          data-node-id="478:1678"
          data-name="Header / Mission"
        >
          <div
            className="relative shrink-0 size-[34.07px]"
            data-node-id="478:1679"
            data-name="Instance / Star"
          >
            <div className="absolute inset-[-2.94%]">
              <img
                alt=""
                className="block max-w-none size-full"
                height="36.07"
                src={imgInstanceStar1}
                width="36.07"
              />
            </div>
          </div>
          <h2
            className="[word-break:break-word] font-display leading-[normal] not-italic relative shrink-0 text-[32px] text-black tracking-[1.6px] whitespace-nowrap"
            data-node-id="478:1681"
          >
            {text(`OUR MISSION & VISION`)}
          </h2>
          <div
            className="relative shrink-0 size-[34.07px]"
            data-node-id="478:1682"
            data-name="Instance / Star / 02"
          >
            <div className="absolute inset-[-2.94%]">
              <img
                alt=""
                className="block max-w-none size-full"
                height="36.07"
                src={imgInstanceStar1}
                width="36.07"
              />
            </div>
          </div>
        </div>
        <div
          className="content-stretch flex gap-[40px] items-center relative shrink-0 w-full"
          data-node-id="478:1684"
          data-name="Container / Mission Split Grid"
        >
          <div
            className="[word-break:break-word] content-stretch flex flex-[1_0_0] flex-col gap-[24px] items-start min-w-px relative text-black"
            data-node-id="478:1685"
            data-name="Container / Left Text Block"
          >
            <div
              className="bg-[rgba(151,143,110,0.18)] content-stretch flex flex-col gap-[16px] items-center p-[32px] relative rounded-[12px] shrink-0 w-full"
              data-node-id="478:1686"
              data-name="Container / Mission Statement Box"
            >
              <p
                className="font-serif font-semibold leading-[normal] relative shrink-0 text-[18px] tracking-[0.72px] whitespace-nowrap"
                data-node-id="478:1687"
              >
                {text("The Heart of Staryield ")}
              </p>
              <p
                className="font-serif font-light leading-[1.4] min-w-full relative shrink-0 text-[24px] text-center w-[min-content]"
                data-node-id="478:1688"
              >
                {text(
                  `"Our mission is to foster self-discovery, spiritual wellness, and genuine connection — in life, love, and within."`,
                )}
              </p>
            </div>
            <p
              className="font-serif font-normal leading-[1.6] relative shrink-0 text-[18px] w-full"
              data-node-id="478:1689"
            >
              {text(
                "Staryield creates a welcoming space for your spiritual journey, offering tools built around you. We strip away the intimidating barriers of traditional esoteric studies, delivering elegant resources that feel intimate and relevant to your modern life. ",
              )}
            </p>
          </div>
          <div
            className="content-stretch flex flex-[1_0_0] flex-col gap-[24px] items-start min-w-px relative"
            data-node-id="478:1690"
            data-name="Container / Right Text Block"
          >
            <p
              className="[word-break:break-word] font-serif font-normal leading-[1.5] relative shrink-0 text-[20px] text-black w-full"
              data-node-id="478:1691"
            >
              {text(
                `We're with you every step of the way, offering insight and guidance whenever you need it — whether you're seeking clarity in life, curious about your future, or simply reflecting.`,
              )}
            </p>
            <div
              className="h-0 relative shrink-0 w-full"
              data-node-id="478:1692"
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
            <p
              className="[word-break:break-word] font-serif font-normal leading-[1.6] relative shrink-0 text-[16px] text-black w-full"
              data-node-id="478:1693"
            >
              {text(
                "Every card pulled, natal chart rendered, and chat session started is backed by advisors who prioritize compassionate listening and absolute spiritual integrity. We do not provide cookie-cutter outputs; we honor your personal truth. ",
              )}
            </p>
          </div>
        </div>
      </section>
      <section
        className="content-stretch flex flex-col h-[670px] items-center justify-between px-6 md:px-10 lg:px-20 py-14 lg:py-[100px] relative shrink-0 w-full figma-section"
        data-node-id="478:1694"
        data-name="Section / Advisors"
      >
        <div aria-hidden className="absolute inset-0 pointer-events-none">
          <div className="absolute bg-[var(--cream)] inset-0" />
          <img
            alt=""
            className="absolute max-w-none mix-blend-overlay object-cover opacity-57 size-full"
            src={imgMediaAbout}
          />
        </div>
        <div
          className="content-stretch flex flex-col gap-[27px] items-center relative shrink-0 w-full"
          data-node-id="478:1695"
          data-name="Layout / Advisors"
        >
          <div
            className="bg-[rgba(170,144,99,0.17)] border border-[#c29a3d] border-solid content-stretch flex items-start px-[12px] py-[6px] relative rounded-[99px] shrink-0"
            data-node-id="478:1696"
            data-name="Section / Badge"
          >
            <p
              className="[word-break:break-word] font-serif font-semibold leading-[normal] relative shrink-0 text-[#c29a3d] text-[12px] uppercase whitespace-nowrap"
              data-node-id="478:1697"
            >
              {text("OUR ADVISORY BOARD ")}
            </p>
          </div>
          <p
            className="[word-break:break-word] font-display leading-[normal] not-italic relative shrink-0 text-[36px] text-black text-center whitespace-nowrap"
            data-node-id="478:1698"
          >
            {text(`Compassionate Listening & Spiritual Integrity`)}
          </p>
          <p
            className="[word-break:break-word] font-serif font-normal leading-[1.6] relative shrink-0 text-[16px] text-black text-center w-[720px]"
            data-node-id="478:1699"
          >
            {text(
              "Our advisors represent the highest standard of esoteric rigor. Every specialist undergoes strict vetting to ensure your personal readings are treated with absolute discretion, empathy, and professional integrity. ",
            )}
          </p>
        </div>
        <div
          className="content-stretch flex gap-[24px] items-start relative shrink-0 w-full"
          data-node-id="478:1700"
          data-name="Layout / Advisors / 02"
        >
          <div
            className="bg-[rgba(170,144,99,0.17)] border border-[rgba(194,154,61,0.15)] border-solid content-stretch flex flex-[1_0_0] flex-col gap-[16px] h-[219px] items-start min-w-px p-[32px] relative rounded-[16px]"
            data-node-id="478:1701"
            data-name="Content Group / Empathetic Guidance"
          >
            <div
              className="relative shrink-0 size-[24px]"
              data-node-id="478:1702"
              data-name="Instance / Star"
            >
              <div className="absolute inset-[-4.17%]">
                <img
                  alt=""
                  className="block max-w-none size-full"
                  height="26"
                  src={imgInstanceStar2}
                  width="26"
                />
              </div>
            </div>
            <p
              className="[word-break:break-word] font-serif font-semibold leading-[normal] relative shrink-0 text-[#c29a3d] text-[20px] whitespace-nowrap"
              data-node-id="478:1704"
            >
              {text("Empathetic Guidance ")}
            </p>
            <p
              className="[word-break:break-word] font-serif font-normal leading-[1.6] min-w-full relative shrink-0 text-[15px] text-black w-[min-content]"
              data-node-id="478:1705"
            >
              {text(
                `No judgment or cookie-cutter scripts. Direct answers to life's most complex equations.`,
              )}
            </p>
          </div>
          <div
            className="bg-[rgba(170,144,99,0.17)] border border-[rgba(194,154,61,0.15)] border-solid content-stretch flex flex-[1_0_0] flex-col gap-[16px] items-start min-w-px p-[32px] relative rounded-[16px]"
            data-node-id="478:1706"
            data-name="Content Group / Rigorous Standards"
          >
            <div
              className="relative shrink-0 size-[24px]"
              data-node-id="478:1707"
              data-name="Instance / Star"
            >
              <div className="absolute inset-[-4.17%]">
                <img
                  alt=""
                  className="block max-w-none size-full"
                  height="26"
                  src={imgInstanceStar2}
                  width="26"
                />
              </div>
            </div>
            <p
              className="[word-break:break-word] font-serif font-semibold leading-[normal] relative shrink-0 text-[#c29a3d] text-[20px] whitespace-nowrap"
              data-node-id="478:1709"
            >
              {text("Rigorous Standards ")}
            </p>
            <p
              className="[word-break:break-word] font-serif font-normal leading-[1.6] min-w-full relative shrink-0 text-[15px] text-black w-[min-content]"
              data-node-id="478:1710"
            >
              {text(
                "Every advisor undergoes continuous audit and feedback to guarantee absolute professional focus. ",
              )}
            </p>
          </div>
          <div
            className="bg-[rgba(170,144,99,0.17)] border border-[rgba(194,154,61,0.15)] border-solid content-stretch flex flex-[1_0_0] flex-col gap-[16px] h-[219px] items-start min-w-px p-[32px] relative rounded-[16px]"
            data-node-id="478:1711"
            data-name="Content Group / Total Confidentiality"
          >
            <div
              className="relative shrink-0 size-[24px]"
              data-node-id="478:1712"
              data-name="Instance / Star"
            >
              <div className="absolute inset-[-4.17%]">
                <img
                  alt=""
                  className="block max-w-none size-full"
                  height="26"
                  src={imgInstanceStar2}
                  width="26"
                />
              </div>
            </div>
            <p
              className="[word-break:break-word] font-serif font-semibold leading-[normal] relative shrink-0 text-[#c29a3d] text-[20px] whitespace-nowrap"
              data-node-id="478:1714"
            >
              {text("Total Confidentiality ")}
            </p>
            <p
              className="[word-break:break-word] font-serif font-normal leading-[1.6] min-w-full relative shrink-0 text-[15px] text-black w-[min-content]"
              data-node-id="478:1715"
            >
              {text(
                "Your dates, cards, and transits are shielded with end-to-end security protocols. ",
              )}
            </p>
          </div>
        </div>
      </section>
      <ContentLink
        className="bg-[var(--cream)] border border-[rgba(194,154,61,0.15)] border-solid content-stretch flex flex-col h-[484px] items-center justify-between overflow-clip px-[100px] py-[80px] relative shrink-0 w-full"
        data-node-id="478:1716"
        data-name="Button / SEEK YOUR ALIGNMENT"
        label="SEEK YOUR ALIGNMENT Curious to know more? Find us on social media. Or explore our FAQ for answers to your questions. Feel free to get in touch with us if you need any help. Our support team and master astrologers are always within reach. Follow our channels for daily transits, zodiac insight, and custom spiritual rituals. Interactive Guidance Get live chats with psychics, order rituals, and schedule personalized sessions. START TODAY"
      >
        <span
          className="content-stretch flex gap-[16px] items-center relative shrink-0"
          data-node-id="478:1717"
          data-name="Button / SEEK YOUR ALIGNMENT"
        >
          <div
            className="relative shrink-0 size-[34.07px]"
            data-node-id="478:1718"
            data-name="Instance / Star"
          >
            <div className="absolute inset-[-1.47%]">
              <img
                alt=""
                className="block max-w-none size-full"
                height="35.07"
                src={imgInstanceStar3}
                width="35.07"
              />
            </div>
          </div>
          <p
            className="[word-break:break-word] font-display leading-[normal] not-italic relative shrink-0 text-[40px] text-black tracking-[2px] whitespace-nowrap"
            data-node-id="478:1720"
          >
            {text("SEEK YOUR ALIGNMENT ")}
          </p>
          <div
            className="relative shrink-0 size-[34.07px]"
            data-node-id="478:1721"
            data-name="Instance / Star / 02"
          >
            <div className="absolute inset-[-1.47%]">
              <img
                alt=""
                className="block max-w-none size-full"
                height="35.07"
                src={imgInstanceStar3}
                width="35.07"
              />
            </div>
          </div>
        </span>
        <div
          className="content-stretch flex gap-[64px] items-center relative shrink-0 w-full"
          data-node-id="478:1723"
          data-name="Container / Content Split"
        >
          <div
            className="[word-break:break-word] content-stretch flex flex-[1_0_0] flex-col gap-[20px] items-start min-w-px relative text-[#0a0a0a]"
            data-node-id="478:1724"
            data-name="Container / Inquiry Info"
          >
            <p
              className="font-serif font-semibold leading-[1.4] relative shrink-0 text-[24px] w-full"
              data-node-id="478:1725"
            >
              {text(
                "Curious to know more? Find us on social media. Or explore our FAQ for answers to your questions. Feel free to get in touch with us if you need any help. ",
              )}
            </p>
            <p
              className="font-serif font-normal leading-[1.6] relative shrink-0 text-[16px] w-full"
              data-node-id="478:1726"
            >
              {text(
                "Our support team and master astrologers are always within reach. Follow our channels for daily transits, zodiac insight, and custom spiritual rituals. ",
              )}
            </p>
          </div>
          <div
            className="content-stretch flex flex-[1_0_0] flex-col gap-[20px] items-start min-w-[400px] p-[24px] relative rounded-[12px]"
            data-node-id="478:1727"
            data-name="Media / Content Split"
          >
            <div
              aria-hidden
              className="absolute inset-0 pointer-events-none rounded-[12px]"
            >
              <div className="absolute bg-[var(--cream)] inset-0 rounded-[12px]" />
              <img
                alt=""
                className="absolute max-w-none mix-blend-overlay object-cover opacity-66 rounded-[12px] size-full"
                src={imgMediaAbout}
              />
            </div>
            <p
              className="[word-break:break-word] font-display leading-[normal] not-italic relative shrink-0 text-[#171717] text-[18px] text-center tracking-[0.54px] w-full"
              data-node-id="478:1728"
            >
              {text("Interactive Guidance ")}
            </p>
            <p
              className="[word-break:break-word] font-serif font-normal leading-[normal] relative shrink-0 text-[#171717] text-[13px] text-center w-full"
              data-node-id="478:1729"
            >
              {text(
                "Get live chats with psychics, order rituals, and schedule personalized sessions. ",
              )}
            </p>
            <div
              className="h-0 relative shrink-0 w-full"
              data-node-id="478:1730"
              data-name="Divider / Horizontal"
            >
              <div className="absolute inset-[-1px_0_0_0]">
                <img
                  alt=""
                  className="block max-w-none size-full"
                  src={imgDividerHorizontal1}
                />
              </div>
            </div>
            <span
              className="border border-[#171717] border-solid content-stretch cursor-pointer flex flex-col h-[62px] items-center justify-center overflow-clip px-[219px] py-[12px] relative rounded-[8px] shrink-0 w-full"
              data-node-id="478:1731"
              data-name="button"
            >
              <div
                aria-hidden
                className="absolute inset-0 pointer-events-none rounded-[8px]"
              >
                <div className="absolute bg-[var(--amber)] inset-0 rounded-[8px]" />
                <img
                  alt=""
                  className="absolute max-w-none mix-blend-overlay object-cover rounded-[8px] size-full"
                  src={imgMediaAbout}
                />
              </div>
              <p
                className="[word-break:break-word] font-display leading-[normal] not-italic relative shrink-0 text-[24px] text-black text-center tracking-[1.2px] whitespace-nowrap"
                data-node-id="I478:1731;113:303"
              >
                {text("START TODAY ")}
              </p>
            </span>
          </div>
        </div>
      </ContentLink>
    </div>
  );
}

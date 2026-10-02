"use client";
import { useI18n } from "@/i18n/i18n-provider";
/* Figma 461:1616 — reusable responsive content regions. */
/* eslint-disable @next/next/no-img-element */
import { ContentLink } from "@/components/content/content-link";
const assetPathPrefix = "https://assets.staryield.net/assets/figma";
const imgMediaHome = `${assetPathPrefix}/fb9ee.jpg`;
const imgCardTheMoon = `${assetPathPrefix}/c829e.png`;
const imgCardTheSun = `${assetPathPrefix}/9e1aa.png`;
const imgCardTheStar = `${assetPathPrefix}/5696a.png`;
const imgMediaR = `${assetPathPrefix}/47a9f.png`;
const imgMediaT = `${assetPathPrefix}/1f3a8.png`;
const imgMediaS = `${assetPathPrefix}/54c44.png`;
const imgDividerHorizontal = `${assetPathPrefix}/7bd7f.svg`;
const imgContainerStar = `${assetPathPrefix}/d9e90.svg`;
const imgDividerHorizontal1 = `${assetPathPrefix}/31cc8.svg`;
const imgContainerCheck = `${assetPathPrefix}/9faa9.svg`;
const imgContainerCheck1 = `${assetPathPrefix}/4cf6a.svg`;
const imgDividerHorizontal2 = `${assetPathPrefix}/90584.svg`;
export default function ScreenDesktopHome() {
  const { text } = useI18n();
  return (
    <div
      className="figma-content flex w-full flex-col items-center bg-cream"
      data-node-id="461:1616"
      data-name="Screen / Desktop / Home"
    >
      <section
        className="bg-[var(--cream)] content-stretch flex items-center justify-between overflow-clip px-6 md:px-10 lg:px-20 py-14 lg:py-[120px] relative shrink-0 w-full figma-section"
        data-node-id="461:1638"
        data-name="Section / Hero"
      >
        <div
          className="content-stretch flex flex-[1_0_0] flex-col gap-[32px] items-start min-w-px relative"
          data-node-id="461:1640"
          data-name="Container / Hero Left Block"
        >
          <div
            className="content-stretch flex gap-[8px] items-center relative shrink-0"
            data-node-id="461:1641"
            data-name="Control / Live Advisors Available 24/7"
          >
            <div
              className="bg-[var(--amber)] relative rounded-[4px] shrink-0 size-[8px]"
              data-node-id="461:1642"
              data-name="Layout / 7"
            />
            <p
              className="[word-break:break-word] font-serif font-semibold leading-[normal] relative shrink-0 text-[14px] text-[color:var(--amber)] uppercase whitespace-nowrap"
              data-node-id="461:1643"
            >
              {text("Live Advisors Available 24/7 ")}
            </p>
          </div>
          <h1
            className="[word-break:break-word] font-display leading-[0] min-w-full not-italic relative shrink-0 text-[56px] text-black w-[min-content]"
            data-node-id="461:1644"
          >
            <span className="leading-[1.15] mb-0">
              {text("Grounded wisdom.")}
            </span>
            <span className="leading-[1.15]">
              {text("Personalized guidance.")}
            </span>
          </h1>
          <p
            className="[word-break:break-word] font-serif font-normal leading-[0] relative shrink-0 text-[18px] text-black tracking-[0.9px] w-[580px]"
            data-node-id="461:1645"
          >
            <span className="font-serif font-semibold leading-[normal]">
              {text("STARYIELD ")}
            </span>
            <span className="leading-[normal]">
              {text(
                ` is a spiritual guidance space where you'll find personalized tools designed to support your self-discovery and nurture your spiritual well-being. When life feels uncertain or overwhelming, our psychics are always here to offer clarity, heartfelt insight, and a genuine sense of connection.`,
              )}
            </span>
          </p>
          <div
            className="content-stretch cursor-pointer flex gap-[16px] items-start relative shrink-0"
            data-node-id="461:1646"
            data-name="Control / Explore Advisors"
          >
            <ContentLink
              className="content-stretch flex items-center justify-center px-[24px] py-[14px] relative rounded-[8px] shrink-0"
              data-node-id="461:1647"
              data-name="Button / Primary / Explore Advisors"
              label="Explore Advisors"
            >
              <div
                aria-hidden
                className="absolute inset-0 pointer-events-none rounded-[8px]"
              >
                <div className="absolute bg-[#c29a3d] inset-0 rounded-[8px]" />
                <img
                  alt=""
                  className="absolute max-w-none mix-blend-overlay object-cover rounded-[8px] size-full"
                  src={imgMediaHome}
                />
              </div>
              <p
                className="[word-break:break-word] font-serif font-black leading-[normal] relative shrink-0 text-[16px] text-left text-white whitespace-nowrap"
                data-node-id="461:1648"
              >
                {text("Explore Advisors ")}
              </p>
            </ContentLink>
            <ContentLink
              className="bg-[#e0d5c3] border border-black border-solid content-stretch flex items-center justify-center px-[24px] py-[14px] relative rounded-[8px] shrink-0"
              data-node-id="461:1649"
              data-name="Button / Secondary / get your astrological calculation"
              label="get your astrological calculation"
            >
              <p
                className="[word-break:break-word] font-serif font-normal leading-[normal] relative shrink-0 text-[16px] text-black text-left whitespace-nowrap"
                data-node-id="461:1650"
              >
                {text("get your astrological calculation ")}
              </p>
            </ContentLink>
          </div>
        </div>
        <div
          className="h-[416px] relative shrink-0 w-[500px]"
          data-node-id="548:1786"
          data-name="Card / Tarot Fan"
        >
          <div
            className="absolute flex h-[375.552px] items-center justify-center left-[258px] top-[83px] w-[284.289px]"
            data-node-id="539:1858"
          >
            <div className="flex-none rotate-15">
              <div
                className="border-[1.5px] border-[var(--gold)] border-solid h-[333.911px] relative rounded-[12px] shadow-[-4px_8px_16px_0px_rgba(0,0,0,0.25)] w-[204.846px]"
                data-name="Card / The Moon"
              >
                <div className="absolute inset-0 overflow-hidden pointer-events-none rounded-[12px]">
                  <img
                    alt=""
                    className="absolute h-[99.66%] left-0 max-w-none top-[0.33%] w-full"
                    src={imgCardTheMoon}
                  />
                </div>
              </div>
            </div>
          </div>
          <div
            className="absolute flex h-[371.904px] items-center justify-center left-[-45px] top-[87px] w-[286.428px]"
            data-node-id="539:1859"
          >
            <div className="flex-none rotate-[-15.77deg]">
              <div
                className="border-[1.5px] border-[var(--gold)] border-solid h-[328.607px] relative rounded-[12px] shadow-[4px_8px_16px_0px_rgba(0,0,0,0.25)] w-[204.846px]"
                data-name="Card / The Sun"
              >
                <img
                  alt=""
                  className="absolute inset-0 max-w-none object-cover pointer-events-none rounded-[12px] size-full"
                  src={imgCardTheSun}
                />
              </div>
            </div>
          </div>
          <div
            className="absolute border-2 border-[var(--gold)] border-solid h-[371px] left-[137.05px] right-[140.95px] rounded-[12px] shadow-[0px_12px_24px_0px_rgba(0,0,0,0.35)] top-[12px]"
            data-node-id="539:1860"
            data-name="Card / The Star"
          >
            <div className="absolute inset-0 overflow-hidden pointer-events-none rounded-[12px]">
              <img
                alt=""
                className="absolute h-[104.33%] left-[-3.49%] max-w-none top-[-2.18%] w-[102.63%]"
                src={imgCardTheStar}
              />
            </div>
          </div>
        </div>
      </section>
      <section
        className="bg-[var(--sage)] border border-[#978f6e] border-solid content-stretch flex flex-col gap-[48px] items-center px-6 md:px-10 lg:px-20 py-14 lg:py-[100px] relative shrink-0 w-full figma-section"
        data-node-id="461:1669"
        data-name="Section / Our Psychics"
      >
        <div
          className="content-stretch flex flex-col gap-[16px] items-center relative shrink-0 w-full"
          data-node-id="461:1670"
          data-name="Layout / Our Psychics"
        >
          <div
            className="bg-[var(--sage)] border border-[#e0d5c3] border-solid content-stretch flex items-start px-[12px] py-[6px] relative rounded-[99px] shrink-0"
            data-node-id="461:1671"
            data-name="Section / Badge"
          >
            <p
              className="[word-break:break-word] font-serif font-semibold leading-[normal] relative shrink-0 text-[#e0d5c3] text-[12px] uppercase whitespace-nowrap"
              data-node-id="461:1672"
            >
              {text("Vetted Specialists ")}
            </p>
          </div>
          <p
            className="[word-break:break-word] font-display leading-[normal] not-italic relative shrink-0 text-[32px] text-center text-white tracking-[1.6px] whitespace-nowrap"
            data-node-id="461:1673"
          >
            {text("Psychic readings built around your journey ")}
          </p>
          <p
            className="[word-break:break-word] font-serif font-normal leading-[normal] relative shrink-0 text-[16px] text-center text-white w-[620px]"
            data-node-id="461:1674"
          >
            {text(
              `We've replaced mystical abstractions with transparent, methodical advice. Select from our top-rated specialists for real-time 1-on-1 consultations.`,
            )}
          </p>
        </div>
        <div
          className="content-stretch flex gap-[105px] items-start relative shrink-0"
          data-node-id="461:1675"
          data-name="Layout / Our Psychics / 02"
        >
          <div
            className="bg-[#e0d5c3] content-stretch flex flex-col gap-[20px] items-start p-[24px] relative rounded-[16px] shrink-0 w-[330px]"
            data-node-id="461:1676"
            data-name="Content Group / R"
          >
            <div
              className="content-stretch flex items-center justify-between relative shrink-0 w-full"
              data-node-id="461:1677"
              data-name="Control / R"
            >
              <div
                className="content-stretch flex items-center justify-center overflow-clip relative rounded-[24px] shrink-0 size-[48px]"
                data-node-id="461:1678"
                data-name="Media / R"
              >
                <img
                  alt=""
                  className="absolute inset-0 max-w-none object-cover pointer-events-none rounded-[24px] size-full"
                  src={imgMediaR}
                />
              </div>
              <div
                className="bg-[rgba(170,142,98,0.15)] content-stretch flex items-start px-[10px] py-[4px] relative rounded-[4px] shrink-0"
                data-node-id="461:1680"
                data-name="Control / Active Now"
              >
                <p
                  className="[word-break:break-word] font-serif font-semibold leading-[normal] relative shrink-0 text-[#aa8e62] text-[12px] whitespace-nowrap"
                  data-node-id="461:1681"
                >
                  {text("Active Now ")}
                </p>
              </div>
            </div>
            <div
              className="[word-break:break-word] content-stretch flex flex-col gap-[4px] items-start leading-[normal] relative shrink-0 text-black w-full whitespace-nowrap"
              data-node-id="461:1682"
              data-name="Control / Ramone"
            >
              <p
                className="font-serif font-semibold relative shrink-0 text-[22px]"
                data-node-id="461:1683"
              >
                {text("Ramone ")}
              </p>
              <p
                className="font-serif font-light relative shrink-0 text-[14px]"
                data-node-id="461:1684"
              >
                {text(`Spiritual Strategy & Intuitive Insights`)}
              </p>
            </div>
            <div
              className="h-0 relative shrink-0 w-full"
              data-node-id="461:1685"
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
            <div
              className="content-stretch flex flex-col gap-[8px] items-start relative shrink-0 w-full"
              data-node-id="461:1686"
              data-name="Control / Consultation Rate"
            >
              <div
                className="[word-break:break-word] content-stretch flex items-start justify-between leading-[normal] relative shrink-0 text-[13px] text-black w-full whitespace-nowrap"
                data-node-id="461:1687"
                data-name="Control / Consultation Rate"
              >
                <p
                  className="font-serif font-light relative shrink-0"
                  data-node-id="461:1688"
                >
                  {text("Consultation Rate ")}
                </p>
                <p
                  className="font-serif font-semibold relative shrink-0"
                  data-node-id="461:1689"
                >
                  {text("$2.99 / min ")}
                </p>
              </div>
              <div
                className="content-stretch flex items-start justify-between relative shrink-0 w-full"
                data-node-id="461:1690"
                data-name="Control / Rating"
              >
                <p
                  className="[word-break:break-word] font-serif font-light leading-[normal] relative shrink-0 text-[13px] text-black whitespace-nowrap"
                  data-node-id="461:1691"
                >
                  {text("Rating ")}
                </p>
                <div
                  className="content-stretch flex gap-[4px] items-center relative shrink-0"
                  data-node-id="461:1692"
                  data-name="Control / 4.9 (420+ reviews)"
                >
                  <div
                    className="relative shrink-0 size-[12px]"
                    data-node-id="461:1858"
                    data-name="Container / Star"
                  >
                    <img
                      alt=""
                      className="absolute block inset-0 max-w-none size-full"
                      src={imgContainerStar}
                    />
                  </div>
                  <p
                    className="[word-break:break-word] font-serif font-semibold leading-[normal] relative shrink-0 text-[13px] text-black whitespace-nowrap"
                    data-node-id="461:1694"
                  >
                    {text("4.9 (420+ reviews) ")}
                  </p>
                </div>
              </div>
            </div>
            <ContentLink
              className="content-stretch cursor-pointer flex items-start justify-center py-[12px] relative rounded-[8px] shrink-0 w-full"
              data-node-id="461:1695"
              data-name="Media / R"
              label="Start 1-on-1 Chat"
            >
              <div
                aria-hidden
                className="absolute inset-0 pointer-events-none rounded-[8px]"
              >
                <div className="absolute bg-[var(--amber)] inset-0 rounded-[8px]" />
                <img
                  alt=""
                  className="absolute max-w-none mix-blend-overlay object-cover rounded-[8px] size-full"
                  src={imgMediaHome}
                />
              </div>
              <p
                className="[word-break:break-word] font-serif font-semibold leading-[normal] relative shrink-0 text-[14px] text-black text-left whitespace-nowrap"
                data-node-id="461:1696"
              >
                {text("Start 1-on-1 Chat ")}
              </p>
            </ContentLink>
          </div>
          <div
            className="bg-[#e0d5c3] content-stretch flex flex-col gap-[20px] items-start p-[24px] relative rounded-[16px] shrink-0 w-[330px]"
            data-node-id="461:1697"
            data-name="Content Group / T"
          >
            <div
              className="content-stretch flex items-center justify-between relative shrink-0 w-full"
              data-node-id="461:1698"
              data-name="Control / T"
            >
              <div
                className="content-stretch flex items-center justify-center overflow-clip relative rounded-[24px] shrink-0 size-[48px]"
                data-node-id="461:1699"
                data-name="Media / T"
              >
                <img
                  alt=""
                  className="absolute inset-0 max-w-none object-cover pointer-events-none rounded-[24px] size-full"
                  src={imgMediaT}
                />
              </div>
              <div
                className="bg-[rgba(170,142,98,0.15)] content-stretch flex items-start px-[10px] py-[4px] relative rounded-[4px] shrink-0"
                data-node-id="461:1701"
                data-name="Control / Active Now"
              >
                <p
                  className="[word-break:break-word] font-serif font-semibold leading-[normal] relative shrink-0 text-[#aa8e62] text-[12px] whitespace-nowrap"
                  data-node-id="461:1702"
                >
                  {text("Active Now ")}
                </p>
              </div>
            </div>
            <div
              className="[word-break:break-word] content-stretch flex flex-col gap-[4px] items-start leading-[normal] relative shrink-0 text-black w-full whitespace-nowrap"
              data-node-id="461:1703"
              data-name="Control / Theo"
            >
              <p
                className="font-serif font-semibold relative shrink-0 text-[22px]"
                data-node-id="461:1704"
              >
                {text("Theo ")}
              </p>
              <p
                className="font-serif font-light relative shrink-0 text-[14px]"
                data-node-id="461:1705"
              >
                {text(`Relational Alignment & Path Clarification`)}
              </p>
            </div>
            <div
              className="h-0 relative shrink-0 w-full"
              data-node-id="461:1706"
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
            <div
              className="content-stretch flex flex-col gap-[8px] items-start relative shrink-0 w-full"
              data-node-id="461:1707"
              data-name="Control / Consultation Rate"
            >
              <div
                className="[word-break:break-word] content-stretch flex items-start justify-between leading-[normal] relative shrink-0 text-[13px] text-black w-full whitespace-nowrap"
                data-node-id="461:1708"
                data-name="Control / Consultation Rate"
              >
                <p
                  className="font-serif font-light relative shrink-0"
                  data-node-id="461:1709"
                >
                  {text("Consultation Rate ")}
                </p>
                <p
                  className="font-serif font-semibold relative shrink-0"
                  data-node-id="461:1710"
                >
                  {text("$3.49 / min ")}
                </p>
              </div>
              <div
                className="content-stretch flex items-start justify-between relative shrink-0 w-full"
                data-node-id="461:1711"
                data-name="Control / Rating"
              >
                <p
                  className="[word-break:break-word] font-serif font-light leading-[normal] relative shrink-0 text-[13px] text-black whitespace-nowrap"
                  data-node-id="461:1712"
                >
                  {text("Rating ")}
                </p>
                <div
                  className="content-stretch flex gap-[4px] items-center relative shrink-0"
                  data-node-id="461:1713"
                  data-name="Control / 4.8 (380+ reviews)"
                >
                  <div
                    className="relative shrink-0 size-[12px]"
                    data-node-id="461:1864"
                    data-name="Container / Star"
                  >
                    <img
                      alt=""
                      className="absolute block inset-0 max-w-none size-full"
                      src={imgContainerStar}
                    />
                  </div>
                  <p
                    className="[word-break:break-word] font-serif font-semibold leading-[normal] relative shrink-0 text-[13px] text-black whitespace-nowrap"
                    data-node-id="461:1715"
                  >
                    {text("4.8 (380+ reviews) ")}
                  </p>
                </div>
              </div>
            </div>
            <ContentLink
              className="content-stretch cursor-pointer flex items-start justify-center py-[12px] relative rounded-[8px] shrink-0 w-full"
              data-node-id="461:1716"
              data-name="Media / T"
              label="Start 1-on-1 Chat"
            >
              <div
                aria-hidden
                className="absolute inset-0 pointer-events-none rounded-[8px]"
              >
                <div className="absolute bg-[var(--amber)] inset-0 rounded-[8px]" />
                <img
                  alt=""
                  className="absolute max-w-none mix-blend-overlay object-cover rounded-[8px] size-full"
                  src={imgMediaHome}
                />
              </div>
              <p
                className="[word-break:break-word] font-serif font-semibold leading-[normal] relative shrink-0 text-[14px] text-black text-left whitespace-nowrap"
                data-node-id="461:1717"
              >
                {text("Start 1-on-1 Chat ")}
              </p>
            </ContentLink>
          </div>
          <div
            className="bg-[#e0d5c3] content-stretch flex flex-col gap-[20px] items-start p-[24px] relative rounded-[16px] shrink-0 w-[330px]"
            data-node-id="461:1718"
            data-name="Content Group / S"
          >
            <div
              className="content-stretch flex items-center justify-between relative shrink-0 w-full"
              data-node-id="461:1719"
              data-name="Control / S"
            >
              <div
                className="content-stretch flex items-center justify-center overflow-clip relative rounded-[24px] shrink-0 size-[48px]"
                data-node-id="461:1720"
                data-name="Media / S"
              >
                <img
                  alt=""
                  className="absolute inset-0 max-w-none object-cover pointer-events-none rounded-[24px] size-full"
                  src={imgMediaS}
                />
              </div>
              <div
                className="bg-[rgba(151,143,110,0.15)] content-stretch flex items-start px-[10px] py-[4px] relative rounded-[4px] shrink-0"
                data-node-id="461:1722"
                data-name="Control / Busy"
              >
                <p
                  className="[word-break:break-word] font-serif font-semibold leading-[normal] relative shrink-0 text-[#978f6e] text-[12px] whitespace-nowrap"
                  data-node-id="461:1723"
                >
                  {text("Busy ")}
                </p>
              </div>
            </div>
            <div
              className="[word-break:break-word] content-stretch flex flex-col gap-[4px] items-start leading-[normal] relative shrink-0 text-black w-full whitespace-nowrap"
              data-node-id="461:1724"
              data-name="Control / Solomon"
            >
              <p
                className="font-serif font-semibold relative shrink-0 text-[22px]"
                data-node-id="461:1725"
              >
                {text("Solomon ")}
              </p>
              <p
                className="font-serif font-light relative shrink-0 text-[14px]"
                data-node-id="461:1726"
              >
                {text(`Astrological Architecture & Chronology`)}
              </p>
            </div>
            <div
              className="h-0 relative shrink-0 w-full"
              data-node-id="461:1727"
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
            <div
              className="content-stretch flex flex-col gap-[8px] items-start relative shrink-0 w-full"
              data-node-id="461:1728"
              data-name="Control / Consultation Rate"
            >
              <div
                className="[word-break:break-word] content-stretch flex items-start justify-between leading-[normal] relative shrink-0 text-[13px] text-black w-full whitespace-nowrap"
                data-node-id="461:1729"
                data-name="Control / Consultation Rate"
              >
                <p
                  className="font-serif font-light relative shrink-0"
                  data-node-id="461:1730"
                >
                  {text("Consultation Rate ")}
                </p>
                <p
                  className="font-serif font-semibold relative shrink-0"
                  data-node-id="461:1731"
                >
                  {text("$3.99 / min ")}
                </p>
              </div>
              <div
                className="content-stretch flex items-start justify-between relative shrink-0 w-full"
                data-node-id="461:1732"
                data-name="Control / Rating"
              >
                <p
                  className="[word-break:break-word] font-serif font-light leading-[normal] relative shrink-0 text-[13px] text-black whitespace-nowrap"
                  data-node-id="461:1733"
                >
                  {text("Rating ")}
                </p>
                <div
                  className="content-stretch flex gap-[4px] items-center relative shrink-0"
                  data-node-id="461:1734"
                  data-name="Control / 5.0 (510+ reviews)"
                >
                  <div
                    className="relative shrink-0 size-[12px]"
                    data-node-id="461:1867"
                    data-name="Container / Star"
                  >
                    <img
                      alt=""
                      className="absolute block inset-0 max-w-none size-full"
                      src={imgContainerStar}
                    />
                  </div>
                  <p
                    className="[word-break:break-word] font-serif font-semibold leading-[normal] relative shrink-0 text-[13px] text-black whitespace-nowrap"
                    data-node-id="461:1736"
                  >
                    {text("5.0 (510+ reviews) ")}
                  </p>
                </div>
              </div>
            </div>
            <ContentLink
              className="bg-[#e0d5c3] border border-black border-solid content-stretch cursor-pointer flex items-start justify-center py-[12px] relative rounded-[8px] shrink-0 w-full"
              data-node-id="461:1737"
              data-name="Control / Join Waiting List"
              label="Join Waiting List"
            >
              <p
                className="[word-break:break-word] font-serif font-semibold leading-[normal] relative shrink-0 text-[14px] text-black text-left whitespace-nowrap"
                data-node-id="461:1738"
              >
                {text("Join Waiting List ")}
              </p>
            </ContentLink>
          </div>
        </div>
      </section>
      <section
        className="content-stretch flex gap-10 lg:gap-20 items-center px-6 md:px-10 lg:px-20 py-14 lg:py-[120px] relative shrink-0 w-full figma-section"
        data-node-id="461:1739"
        data-name="Section / Overview"
      >
        <div aria-hidden className="absolute inset-0 pointer-events-none">
          <div className="absolute bg-[var(--cream)] inset-0" />
          <img
            alt=""
            className="absolute max-w-none mix-blend-overlay object-cover size-full"
            src={imgMediaHome}
          />
        </div>
        <div
          className="bg-[var(--warm-gray)] content-stretch flex flex-col gap-[20px] h-[276px] items-start p-[32px] relative rounded-[20px] shrink-0 w-[500px]"
          data-node-id="461:1740"
          data-name="Media / Overview"
        >
          <p
            className="[word-break:break-word] font-serif font-medium leading-[normal] relative shrink-0 text-[18px] text-black whitespace-nowrap"
            data-node-id="461:1741"
          >
            {text("Operational Journey Path ")}
          </p>
          <div
            className="content-stretch flex flex-col gap-[12px] items-start relative shrink-0 w-full"
            data-node-id="461:1742"
            data-name="Content Group / 1"
          >
            <div
              className="content-stretch flex gap-[12px] items-center relative shrink-0 w-full"
              data-node-id="461:1743"
              data-name="Control / 1"
            >
              <div
                className="bg-[var(--sage)] content-stretch flex items-center justify-center relative rounded-[12px] shrink-0 size-[24px]"
                data-node-id="461:1744"
                data-name="Control / 1"
              >
                <p
                  className="[word-break:break-word] font-serif font-semibold leading-[normal] relative shrink-0 text-[11px] text-black whitespace-nowrap"
                  data-node-id="461:1745"
                >
                  1
                </p>
              </div>
              <p
                className="[word-break:break-word] font-serif font-normal leading-[normal] relative shrink-0 text-[14px] text-black whitespace-nowrap"
                data-node-id="461:1746"
              >
                {text("Identify Existential Bottlenecks ")}
              </p>
            </div>
            <div
              className="h-0 relative shrink-0 w-full"
              data-node-id="461:1747"
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
            <div
              className="content-stretch flex gap-[12px] items-center relative shrink-0 w-full"
              data-node-id="461:1748"
              data-name="Control / 2"
            >
              <div
                className="bg-[var(--warm-gray)] border border-black border-solid content-stretch flex items-center justify-center relative rounded-[12px] shrink-0 size-[24px]"
                data-node-id="461:1749"
                data-name="Control / 2"
              >
                <p
                  className="[word-break:break-word] font-serif font-semibold leading-[normal] relative shrink-0 text-[11px] text-black whitespace-nowrap"
                  data-node-id="461:1750"
                >
                  2
                </p>
              </div>
              <p
                className="[word-break:break-word] font-serif font-normal leading-[normal] relative shrink-0 text-[14px] text-black whitespace-nowrap"
                data-node-id="461:1751"
              >
                {text("Execute Alignment Consultation ")}
              </p>
            </div>
            <div
              className="h-0 relative shrink-0 w-full"
              data-node-id="461:1752"
              data-name="Divider / Horizontal / 02"
            >
              <div className="absolute inset-[-1px_0_0_0]">
                <img
                  alt=""
                  className="block max-w-none size-full"
                  src={imgDividerHorizontal1}
                />
              </div>
            </div>
            <div
              className="content-stretch flex gap-[12px] items-center relative shrink-0 w-full"
              data-node-id="461:1753"
              data-name="Control / 3"
            >
              <div
                className="bg-[var(--warm-gray)] border border-black border-solid content-stretch flex items-center justify-center relative rounded-[12px] shrink-0 size-[24px]"
                data-node-id="461:1754"
                data-name="Control / 3"
              >
                <p
                  className="[word-break:break-word] font-serif font-semibold leading-[normal] relative shrink-0 text-[11px] text-black whitespace-nowrap"
                  data-node-id="461:1755"
                >
                  3
                </p>
              </div>
              <p
                className="[word-break:break-word] font-serif font-normal leading-[normal] relative shrink-0 text-[14px] text-black whitespace-nowrap"
                data-node-id="461:1756"
              >
                {text("Iterate with Bi-weekly Syntheses ")}
              </p>
            </div>
          </div>
        </div>
        <div
          className="content-stretch flex flex-[1_0_0] flex-col gap-[24px] items-start min-w-px relative"
          data-node-id="461:1757"
          data-name="Container / Overview Right (Benefits)"
        >
          <div
            className="border border-[var(--amber)] border-solid content-stretch flex items-start px-[12px] py-[6px] relative rounded-[99px] shrink-0"
            data-node-id="461:1758"
            data-name="Section / Badge"
          >
            <p
              className="[word-break:break-word] font-serif font-semibold leading-[normal] relative shrink-0 text-[12px] text-[color:var(--amber)] uppercase whitespace-nowrap"
              data-node-id="461:1759"
            >
              {text("Alignment Framework ")}
            </p>
          </div>
          <p
            className="[word-break:break-word] font-display leading-[normal] min-w-full not-italic relative shrink-0 text-[36px] text-black w-[min-content]"
            data-node-id="461:1760"
          >
            {text("Methodical insights built around your personal trajectory ")}
          </p>
          <p
            className="[word-break:break-word] font-serif font-extralight leading-[1.6] min-w-full relative shrink-0 text-[16px] text-black w-[min-content]"
            data-node-id="461:1761"
          >
            {text(
              `Everyone's path to self-discovery is unique. Our platform removes the speculative guesswork and introduces a card-based alignment structure designed for modern execution.`,
            )}
          </p>
          <div
            className="content-stretch flex flex-col gap-[16px] items-start pt-[16px] relative shrink-0 w-full"
            data-node-id="461:1762"
            data-name="Layout / Overview Right (Benefits)"
          >
            <div
              className="content-stretch flex gap-[16px] items-center relative shrink-0"
              data-node-id="461:1763"
              data-name="Control / Convert life challenges into clear, act…"
            >
              <div
                className="relative shrink-0 size-[20px]"
                data-node-id="461:1870"
                data-name="Container / Check"
              >
                <img
                  alt=""
                  className="absolute block inset-0 max-w-none size-full"
                  src={imgContainerCheck}
                />
              </div>
              <p
                className="[word-break:break-word] font-serif font-normal leading-[normal] relative shrink-0 text-[15px] text-black whitespace-nowrap"
                data-node-id="461:1765"
              >
                {text(
                  "Convert life challenges into clear, actionable strategies. ",
                )}
              </p>
            </div>
            <div
              className="content-stretch flex gap-[16px] items-center relative shrink-0"
              data-node-id="461:1766"
              data-name="Control / Understand structural relational patter…"
            >
              <div
                className="relative shrink-0 size-[20px]"
                data-node-id="461:1873"
                data-name="Container / Check"
              >
                <img
                  alt=""
                  className="absolute block inset-0 max-w-none size-full"
                  src={imgContainerCheck1}
                />
              </div>
              <p
                className="[word-break:break-word] font-serif font-normal leading-[normal] relative shrink-0 text-[15px] text-black whitespace-nowrap"
                data-node-id="461:1768"
              >
                {text(
                  "Understand structural relational patterns with complete confidence. ",
                )}
              </p>
            </div>
            <div
              className="content-stretch flex gap-[16px] items-center relative shrink-0"
              data-node-id="461:1769"
              data-name="Control / Locate emotional balance using objectiv…"
            >
              <div
                className="relative shrink-0 size-[20px]"
                data-node-id="461:1876"
                data-name="Container / Check"
              >
                <img
                  alt=""
                  className="absolute block inset-0 max-w-none size-full"
                  src={imgContainerCheck1}
                />
              </div>
              <p
                className="[word-break:break-word] font-serif font-normal leading-[normal] relative shrink-0 text-[15px] text-black whitespace-nowrap"
                data-node-id="461:1771"
              >
                {text(
                  "Locate emotional balance using objective chronologies. ",
                )}
              </p>
            </div>
          </div>
        </div>
      </section>
      <div
        className="bg-[var(--warm-gray)] border border-[#978f6e] border-solid content-stretch flex flex-col gap-[64px] items-center px-6 md:px-10 lg:px-20 py-14 lg:py-[100px] relative shrink-0 w-full"
        data-node-id="461:1772"
        data-name="Container / Testimonials & Stats"
      >
        <div
          className="content-stretch flex flex-col gap-[10px] items-center justify-center relative shrink-0 w-[878px]"
          data-node-id="461:1773"
          data-name="Container / Testimonial Quote"
        >
          <div
            className="[word-break:break-word] content-stretch flex items-center relative shrink-0 text-black w-full"
            data-node-id="450:1576"
            data-name="Layout / Testimonial Quote"
          >
            <p
              className="font-display h-[40px] leading-[normal] not-italic relative shrink-0 text-[36px] w-[36px]"
              data-node-id="461:1774"
            >
              {text(`"`)}
            </p>
            <p
              className="flex-[1_0_0] font-serif font-light leading-[1.5] min-w-px relative text-[24px] text-center"
              data-node-id="461:1775"
            >
              {text(
                "Absolutely precise. I was looking for structural clarity without the typical metaphysical clutter. My advisor mapped out my transition timelines perfectly. This is the exact tool I needed. ",
              )}
            </p>
            <p
              className="font-display h-[40px] leading-[normal] not-italic relative shrink-0 text-[36px] w-[36px]"
              data-node-id="450:1575"
            >
              {text(`"`)}
            </p>
          </div>
          <div
            className="content-stretch flex gap-[12px] items-center relative shrink-0"
            data-node-id="461:1776"
            data-name="Control / H. Reynolds"
          >
            <div
              className="bg-[#c29a3d] relative rounded-[16px] shrink-0 size-[32px]"
              data-node-id="461:1777"
              data-name="Layout / H. Reynolds"
            />
            <p
              className="[word-break:break-word] font-serif font-semibold leading-[normal] relative shrink-0 text-[15px] text-black whitespace-nowrap"
              data-node-id="461:1778"
            >
              {text("H. Reynolds ")}
            </p>
          </div>
        </div>
        <div
          className="content-stretch flex gap-[40px] items-start justify-center p-[32px] relative rounded-[16px] shrink-0 w-full"
          data-node-id="461:1780"
          data-name="Media / Testimonials & Stats"
        >
          <div
            aria-hidden
            className="absolute inset-0 pointer-events-none rounded-[16px]"
          >
            <div className="absolute bg-[#e0d5c3] inset-0 rounded-[16px]" />
            <img
              alt=""
              className="absolute max-w-none mix-blend-overlay object-cover rounded-[16px] size-full"
              src={imgMediaHome}
            />
          </div>
          <div
            className="[word-break:break-word] content-stretch flex flex-[1_0_0] flex-col gap-[4px] items-center leading-[normal] min-w-px relative text-black whitespace-nowrap"
            data-node-id="461:1781"
            data-name="Control / 4.8"
          >
            <p
              className="font-display not-italic relative shrink-0 text-[48px]"
              data-node-id="461:1782"
            >
              4.8
            </p>
            <p
              className="font-serif font-normal relative shrink-0 text-[14px] uppercase"
              data-node-id="461:1783"
            >
              {text("Average Platform Rating ")}
            </p>
          </div>
          <div
            className="flex h-[60px] items-center justify-center relative shrink-0 w-0"
            data-node-id="461:1784"
          >
            <div className="flex-none rotate-90">
              <div
                className="h-0 relative w-[60px]"
                data-name="Divider / Horizontal"
              >
                <div className="absolute inset-[-1px_0_0_0]">
                  <img
                    alt=""
                    className="block max-w-none size-full"
                    src={imgDividerHorizontal2}
                  />
                </div>
              </div>
            </div>
          </div>
          <div
            className="[word-break:break-word] content-stretch flex flex-[1_0_0] flex-col gap-[4px] items-center leading-[normal] min-w-px relative text-black whitespace-nowrap"
            data-node-id="461:1785"
            data-name="Control / 50+"
          >
            <p
              className="font-display not-italic relative shrink-0 text-[48px]"
              data-node-id="461:1786"
            >
              50+
            </p>
            <p
              className="font-serif font-normal relative shrink-0 text-[14px] uppercase"
              data-node-id="461:1787"
            >
              {text("Countries Serviced ")}
            </p>
          </div>
          <div
            className="flex h-[60px] items-center justify-center relative shrink-0 w-0"
            data-node-id="461:1788"
          >
            <div className="flex-none rotate-90">
              <div
                className="h-0 relative w-[60px]"
                data-name="Divider / Horizontal / 02"
              >
                <div className="absolute inset-[-1px_0_0_0]">
                  <img
                    alt=""
                    className="block max-w-none size-full"
                    src={imgDividerHorizontal2}
                  />
                </div>
              </div>
            </div>
          </div>
          <div
            className="[word-break:break-word] content-stretch flex flex-[1_0_0] flex-col gap-[4px] items-center leading-[normal] min-w-px relative text-black whitespace-nowrap"
            data-node-id="461:1789"
            data-name="Control / 10K+"
          >
            <p
              className="font-display not-italic relative shrink-0 text-[48px]"
              data-node-id="461:1790"
            >
              {text("10K+ ")}
            </p>
            <p
              className="font-serif font-normal relative shrink-0 text-[14px] uppercase"
              data-node-id="461:1791"
            >
              {text("Completed Consultations ")}
            </p>
          </div>
        </div>
      </div>
      <section
        className="bg-[var(--sage)] content-stretch flex flex-col gap-[50px] h-[731px] items-center justify-center px-6 md:px-10 lg:px-20 py-14 lg:py-[120px] relative shrink-0 w-full figma-section"
        data-node-id="461:1792"
        data-name="Section / How To Start"
      >
        <div
          className="content-stretch flex flex-col gap-[16px] items-center relative shrink-0 w-full"
          data-node-id="461:1793"
          data-name="Layout / How To Start"
        >
          <div
            className="bg-[rgba(194,154,61,0.13)] border border-solid border-white content-stretch flex items-start px-[12px] py-[6px] relative rounded-[99px] shrink-0"
            data-node-id="461:1794"
            data-name="Section / Badge"
          >
            <p
              className="[word-break:break-word] font-serif font-semibold leading-[normal] relative shrink-0 text-[12px] text-white uppercase whitespace-nowrap"
              data-node-id="461:1795"
            >
              {text("Execution Path ")}
            </p>
          </div>
          <p
            className="[word-break:break-word] font-display leading-[normal] not-italic relative shrink-0 text-[36px] text-white whitespace-nowrap"
            data-node-id="461:1796"
          >
            {text("Onboarding Protocol ")}
          </p>
        </div>
        <div
          className="[word-break:break-word] content-stretch flex gap-[24px] items-start justify-center relative shrink-0 w-full"
          data-node-id="461:1797"
          data-name="Layout / How To Start / 02"
        >
          <div
            className="bg-[var(--cream)] content-stretch flex flex-[1_0_0] flex-col gap-[16px] h-[260px] items-start min-w-px p-[24px] relative rounded-[12px]"
            data-node-id="461:1798"
            data-name="Content Group / 01"
          >
            <p
              className="font-serif font-bold leading-[normal] relative shrink-0 text-[20px] text-[color:var(--amber)] whitespace-nowrap"
              data-node-id="461:1799"
            >
              01
            </p>
            <p
              className="font-serif font-medium leading-[normal] relative shrink-0 text-[18px] text-black whitespace-nowrap"
              data-node-id="461:1800"
            >
              {text("Diagnostic Intake ")}
            </p>
            <p
              className="font-serif font-light leading-[1.5] min-w-full relative shrink-0 text-[14px] text-black w-[min-content]"
              data-node-id="461:1801"
            >
              {text(
                "Complete a brief diagnostic matrix to help us match structural advisors to your career or personal journey. ",
              )}
            </p>
          </div>
          <div
            className="bg-[var(--cream)] content-stretch flex flex-[1_0_0] flex-col gap-[16px] h-[260px] items-start min-w-px p-[24px] relative rounded-[12px]"
            data-node-id="461:1802"
            data-name="Content Group / 02"
          >
            <p
              className="font-serif font-bold leading-[normal] relative shrink-0 text-[20px] text-[color:var(--amber)] whitespace-nowrap"
              data-node-id="461:1803"
            >
              02
            </p>
            <p
              className="font-serif font-medium leading-[normal] relative shrink-0 text-[18px] text-black whitespace-nowrap"
              data-node-id="461:1804"
            >
              {text("Select Advisor ")}
            </p>
            <p
              className="font-serif font-light leading-[1.5] min-w-full relative shrink-0 text-[14px] text-black w-[min-content]"
              data-node-id="461:1805"
            >
              {text(
                "Browse certified specialists, review credentials and analytical ratings, and choose the ideal consultant. ",
              )}
            </p>
          </div>
          <div
            className="bg-[var(--cream)] content-stretch flex flex-[1_0_0] flex-col gap-[16px] h-[260px] items-start min-w-px p-[24px] relative rounded-[12px]"
            data-node-id="461:1806"
            data-name="Content Group / 03"
          >
            <p
              className="font-serif font-bold leading-[normal] relative shrink-0 text-[20px] text-[color:var(--amber)] whitespace-nowrap"
              data-node-id="461:1807"
            >
              03
            </p>
            <p
              className="font-serif font-medium leading-[normal] relative shrink-0 text-[18px] text-black whitespace-nowrap"
              data-node-id="461:1808"
            >
              {text("Initiate Session ")}
            </p>
            <p
              className="font-serif font-light leading-[1.5] min-w-full relative shrink-0 text-[14px] text-black w-[min-content]"
              data-node-id="461:1809"
            >
              {text(
                "Connect through a secure 1-on-1 virtual workstation for a direct, real-time consultation experience. ",
              )}
            </p>
          </div>
          <div
            className="bg-[var(--cream)] content-stretch flex flex-[1_0_0] flex-col gap-[16px] h-[260px] items-start min-w-px p-[24px] relative rounded-[12px]"
            data-node-id="461:1810"
            data-name="Content Group / 04"
          >
            <p
              className="font-serif font-bold leading-[normal] relative shrink-0 text-[20px] text-[color:var(--amber)] whitespace-nowrap"
              data-node-id="461:1811"
            >
              04
            </p>
            <p
              className="font-serif font-medium leading-[normal] relative shrink-0 text-[18px] text-black whitespace-nowrap"
              data-node-id="461:1812"
            >
              {text("Continuous Audit ")}
            </p>
            <p
              className="font-serif font-light leading-[1.5] min-w-full relative shrink-0 text-[14px] text-black w-[min-content]"
              data-node-id="461:1813"
            >
              {text(
                "Log alignment tracking data to audit your trajectory over consecutive monthly cycles. ",
              )}
            </p>
          </div>
        </div>
        <ContentLink
          className="content-stretch cursor-pointer flex items-start relative shrink-0"
          data-node-id="461:1814"
          data-name="Control / Get started"
          label="Get started"
        >
          <span
            className="border-[0.5px] border-black border-solid content-stretch flex items-center justify-center px-[24px] py-[14px] relative rounded-[8px] shrink-0 w-[400px]"
            data-node-id="461:1815"
            data-name="Button / Primary / Get started"
          >
            <div
              aria-hidden
              className="absolute inset-0 pointer-events-none rounded-[8px]"
            >
              <div className="absolute bg-[var(--amber)] inset-0 rounded-[8px]" />
              <img
                alt=""
                className="absolute max-w-none mix-blend-overlay object-cover rounded-[8px] size-full"
                src={imgMediaHome}
              />
            </div>
            <p
              className="[word-break:break-word] font-display leading-[normal] not-italic relative shrink-0 text-[16px] text-black text-left whitespace-nowrap"
              data-node-id="461:1816"
            >
              {text("Get started ")}
            </p>
          </span>
        </ContentLink>
      </section>
    </div>
  );
}

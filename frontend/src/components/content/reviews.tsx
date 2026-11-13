"use client";
import { useI18n } from "@/i18n/i18n-provider";
/* Figma 342:1455 — reusable responsive content regions. */
/* eslint-disable @next/next/no-img-element */
import { useSignupFreeMinutes } from "@/lib/site-config/site-config-provider";
import { ContentLink } from "@/components/content/content-link";
import { COMPANY } from "@/config/company"; // revisions 5.1
import { TrustpilotReviewCollector } from "@/components/reviews/trustpilot-review-collector";
const assetPathPrefix = "https://assets.staryield.net/assets/figma";
const imgMediaReviews = `${assetPathPrefix}/fb9ee.jpg`;
const imgInstanceStar = `${assetPathPrefix}/e7e99.png`;
const imgDividerHorizontal = `${assetPathPrefix}/2e925.svg`;
const imgContainerCheck = `${assetPathPrefix}/ceeda.svg`;
const imgDividerHorizontal1 = `${assetPathPrefix}/5455a.svg`;
export default function ScreenDesktopReviews() {
  const { text } = useI18n();
  const freeMinutes = useSignupFreeMinutes();
  return (
    <div
      className="figma-content flex w-full flex-col items-center bg-cream"
      data-node-id="342:1455"
      data-name="Screen / Desktop / Reviews"
    >
      <div
        className="bg-[var(--cream)] border-[var(--cream)] border-b border-solid content-stretch flex flex-col gap-[16px] min-h-[252px] items-center overflow-clip pb-[64px] pt-[45px] px-[120px] relative shrink-0 w-full"
        data-node-id="342:1476"
        data-name="Container / Review Stats Hero"
      >
        <div
          className="content-stretch flex items-center relative shrink-0"
          data-node-id="342:1477"
          data-name="Control / Trustpilot"
        >
          <h1
            className="[word-break:break-word] font-display leading-[normal] not-italic relative shrink-0 text-[40px] text-black tracking-[1.6px] whitespace-nowrap"
            data-node-id="342:1479"
          >
            {text("Trustpilot ")}
          </h1>
        </div>
        <div
          className="content-stretch flex w-full max-w-[560px] flex-col gap-[16px] items-center relative shrink-0"
          data-name="Content Group / Trustpilot"
        >
          <p className="[word-break:break-word] font-serif font-light leading-[normal] text-[20px] text-black text-center">
            {text(
              "Read what people say about Staryield, or share your own experience.",
            )}
          </p>
          <TrustpilotReviewCollector placement="reviews" />
        </div>
      </div>
      {/* revisions 5.1: the three hard-coded review cards were removed (docs/legal-review-checklist.md row 59); reviews are the Trustpilot widget above. */}
      <div
        className="bg-[var(--sage)] content-stretch flex flex-col gap-[32px] items-start overflow-clip px-[160px] py-[96px] relative shrink-0 w-full"
        data-node-id="342:1550"
        data-name="Container / Editorial Intro"
      >
        <section
          className="content-stretch flex items-center justify-between overflow-clip relative shrink-0 w-full figma-section"
          data-node-id="342:1551"
          data-name="Section / Header"
        >
          <p
            className="[word-break:break-word] font-display leading-[normal] not-italic relative shrink-0 text-[40px] text-center text-white tracking-[1.6px] whitespace-nowrap"
            data-node-id="342:1554"
          >
            {text("Staryield Review ")}
          </p>
          <div
            className="relative shrink-0 size-[34.07px]"
            data-node-id="342:1555"
            data-name="Instance / Star"
          />
          <p
            className="[word-break:break-word] font-serif font-extralight leading-[normal] relative shrink-0 text-[20px] text-center text-white tracking-[0.8px] whitespace-nowrap"
            data-node-id="464:21684"
          >
            {text("Everything You Need to Know ")}
          </p>
        </section>
        <div
          className="[word-break:break-word] content-stretch flex flex-col gap-[24px] items-start relative shrink-0 text-white w-full"
          data-node-id="342:1557"
          data-name="Layout / Editorial Intro"
        >
          <p
            className="font-serif font-medium leading-[1.45] relative shrink-0 text-[28px] w-full"
            data-node-id="342:1559"
          >
            {text(
              "Are you seeking clarity on a relationship, or wondering what direction your life is heading? Maybe you are looking for comfort during a difficult transition. ",
            )}
          </p>
          <p
            className="font-serif font-normal leading-[1.6] relative shrink-0 text-[20px] w-full"
            data-node-id="342:1560"
          >
            {text(
              "What if there was a welcoming space where you could explore fresh perspectives on your past, present, and future? ",
            )}
          </p>
          <p
            className="font-serif font-normal leading-[1.6] relative shrink-0 text-[20px] w-full"
            data-node-id="342:1561"
          >
            {text(
              "That space is - a platform devoted to helping people reflect on their relationships, purpose, and path through personalized psychic and astrology sessions. ",
            )}
          </p>
          <p
            className="font-serif font-normal leading-[1.6] relative shrink-0 text-[20px] w-full"
            data-node-id="342:1562"
          >
            {text(
              "With advisors from many backgrounds and specialties, you can chat with a psychic whenever one is online, to reflect on yourself and the people around you. ",
            )}
          </p>
          <p
            className="font-serif font-normal leading-[1.6] relative shrink-0 text-[20px] w-full"
            data-node-id="342:1563"
          >
            {text(
              "If you are looking for a compassionate, nonjudgmental conversation and want to learn more about yourself and others, Staryield may be for you. Our platform connects you with intuitive advisors. Browse the directory to find the one who feels right for your journey. ",
            )}
          </p>
        </div>
      </div>
      <div
        className="grid-rows-[max-content] inline-grid leading-[0] place-items-start relative shrink-0 w-full"
        data-node-id="784:2138"
        data-name="Group / Reviews"
      >
        <section
          className="bg-[var(--cream)] border-[var(--gold)] border-b border-solid col-1 content-stretch flex flex-col gap-[40px] items-start ml-0 mt-0 overflow-clip px-[160px] py-[80px] relative row-1 w-full figma-section"
          data-node-id="342:1566"
          data-name="Section / Facts"
        >
          <section
            className="content-stretch flex gap-[16px] items-center justify-center relative shrink-0 w-full figma-section"
            data-node-id="342:1567"
            data-name="Section / Header"
          >
            <div
              className="relative shrink-0 size-[34.07px]"
              data-node-id="342:1568"
              data-name="Instance / Star"
            >
              <img
                alt=""
                className="absolute block inset-0 max-w-none size-full"
                height="34.07"
                src={imgInstanceStar}
                width="34.07"
              />
            </div>
            <p
              className="[word-break:break-word] font-display leading-[normal] not-italic relative shrink-0 text-[32px] text-black text-center tracking-[1.28px] whitespace-nowrap"
              data-node-id="342:1570"
            >
              {text("A Few Things to Know ")}
            </p>
            <div
              className="relative shrink-0 size-[34.07px]"
              data-node-id="342:1571"
              data-name="Instance / Star / 02"
            >
              <img
                alt=""
                className="absolute block inset-0 max-w-none size-full"
                height="34.07"
                src={imgInstanceStar}
                width="34.07"
              />
            </div>
          </section>
          <div
            className="[word-break:break-word] content-stretch flex flex-col gap-[24px] items-start relative shrink-0 text-black w-full"
            data-node-id="342:1573"
            data-name="Layout / Facts"
          >
            <p
              className="font-serif font-normal leading-[1.6] min-w-full relative shrink-0 text-[24px] w-[min-content]"
              data-node-id="342:1575"
            >
              {text(
                "If you are wondering whether Staryield is right for you, here are a few things worth knowing: ",
              )}
            </p>
            <div
              className="font-serif font-light grid-cols-[max-content] grid-rows-[max-content] inline-grid leading-[0] place-items-start relative shrink-0 text-[18px]"
              data-node-id="464:22027"
              data-name="Group / Facts"
            >
              <p
                className="col-1 leading-[1.6] ml-0 mt-0 relative row-1 w-[1120px]"
                data-node-id="342:1576"
              >
                {text(
                  `• Support is available 24/7. We reply within 1–2 days.`,
                )}
              </p>
              <p
                className="col-1 leading-[1.6] ml-0 mt-[70px] relative row-1 w-[1120px]"
                data-node-id="342:1577"
              >
                {text(
                  `• Readings are offered for entertainment purposes only.`,
                )}
              </p>
              <p
                className="col-1 leading-[1.6] ml-0 mt-[140px] relative row-1 w-[1120px]"
                data-node-id="342:1578"
              >
                {text(
                  "• Reviews of Staryield are written by independent users on Trustpilot: read them there, or share your own. ",
                )}
              </p>
            </div>
          </div>
        </section>
        <section
          className="bg-[var(--cream)] col-1 content-stretch flex gap-[40px] items-start justify-center ml-0 mt-[536px] overflow-clip px-[160px] py-[80px] relative row-1 w-full figma-section"
          data-node-id="342:1579"
          data-name="Section / Pros"
        >
          <section
            className="content-stretch flex gap-[16px] h-[338px] items-center justify-center overflow-clip relative shrink-0 figma-section"
            data-node-id="342:1580"
            data-name="Section / Header"
          >
            <div
              className="relative shrink-0 size-[34.07px]"
              data-node-id="342:1581"
              data-name="Instance / Star"
            />
            <div
              className="[word-break:break-word] font-display leading-[0] not-italic relative shrink-0 text-[40px] text-black text-center tracking-[1.6px] whitespace-nowrap"
              data-node-id="342:1583"
            >
              <p className="leading-[normal] mb-0 whitespace-pre">
                {text(`Why Choose `)}
              </p>
              <p className="leading-[normal] whitespace-pre">
                {text("Staryield")}
              </p>
            </div>
            <div
              className="relative shrink-0 size-[34.07px]"
              data-node-id="342:1584"
              data-name="Instance / Star / 02"
            />
          </section>
          <div
            className="content-stretch flex flex-[1_0_0] items-center min-w-px relative"
            data-node-id="342:1586"
            data-name="Layout / Pros"
          >
            <div
              className="bg-[var(--warm-gray)] border-2 border-black border-solid content-stretch flex flex-[1_0_0] flex-col gap-[20px] items-start min-w-px p-[32px] relative rounded-[12px]"
              data-node-id="342:1601"
              data-name="Layout / Pros"
            >
              <p
                className="[word-break:break-word] font-display leading-[normal] not-italic relative shrink-0 text-[24px] text-black whitespace-nowrap"
                data-node-id="342:1602"
              >
                {text(`Staryield's Pros`)}
              </p>
              <div
                className="h-0 relative shrink-0 w-full"
                data-node-id="342:1603"
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
                className="content-stretch flex gap-[12px] items-center relative shrink-0 w-full"
                data-node-id="342:1604"
                data-name="Content Group / Convenient 1:1 chat readings available…"
              >
                <div
                  className="bg-[var(--sage)] border border-black border-solid content-stretch flex items-center justify-center relative rounded-[999px] shrink-0 size-[24px]"
                  data-node-id="342:1605"
                  data-name="Layout / Convenient 1:1 Chat Readings Available…"
                >
                  <div
                    className="relative shrink-0 size-[12px]"
                    data-node-id="342:1802"
                    data-name="Container / Check"
                  >
                    <img
                      alt=""
                      className="absolute block inset-0 max-w-none size-full"
                      src={imgContainerCheck}
                    />
                  </div>
                </div>
                <p
                  className="[word-break:break-word] flex-[1_0_0] font-serif font-normal leading-[normal] min-w-px relative text-[18px] text-black"
                  data-node-id="342:1607"
                >
                  {text("Private 1:1 chat readings ")}
                </p>
              </div>
              <div
                className="content-stretch flex gap-[12px] items-center relative shrink-0 w-full"
                data-node-id="342:1608"
                data-name="Content Group / Love and relationship readings"
              >
                <div
                  className="bg-[var(--sage)] border border-black border-solid content-stretch flex items-center justify-center relative rounded-[999px] shrink-0 size-[24px]"
                  data-node-id="342:1609"
                  data-name="Layout / Love And Relationship Readings"
                >
                  <div
                    className="relative shrink-0 size-[12px]"
                    data-node-id="342:1805"
                    data-name="Container / Check"
                  >
                    <img
                      alt=""
                      className="absolute block inset-0 max-w-none size-full"
                      src={imgContainerCheck}
                    />
                  </div>
                </div>
                <p
                  className="[word-break:break-word] flex-[1_0_0] font-serif font-normal leading-[normal] min-w-px relative text-[18px] text-black"
                  data-node-id="342:1611"
                >
                  {text("Readings about love and relationships ")}
                </p>
              </div>
              <div
                className="content-stretch flex gap-[12px] items-center relative shrink-0 w-full"
                data-node-id="342:1612"
                data-name="Content Group / Free horoscopes and personalized astrol…"
              >
                <div
                  className="bg-[var(--sage)] border border-black border-solid content-stretch flex items-center justify-center relative rounded-[999px] shrink-0 size-[24px]"
                  data-node-id="342:1613"
                  data-name="Layout / Free Horoscopes And Personalized Astrol…"
                >
                  <div
                    className="relative shrink-0 size-[12px]"
                    data-node-id="342:1808"
                    data-name="Container / Check"
                  >
                    <img
                      alt=""
                      className="absolute block inset-0 max-w-none size-full"
                      src={imgContainerCheck}
                    />
                  </div>
                </div>
                <p
                  className="[word-break:break-word] flex-[1_0_0] font-serif font-normal leading-[normal] min-w-px relative text-[18px] text-black"
                  data-node-id="342:1615"
                >
                  {text("Free horoscopes and astrology calculators ")}
                </p>
              </div>
              <div
                className="content-stretch flex gap-[12px] items-center relative shrink-0 w-full"
                data-node-id="342:1616"
                data-name="Content Group / Clear prices shown before every reading"
              >
                <div
                  className="bg-[var(--sage)] border border-black border-solid content-stretch flex items-center justify-center relative rounded-[999px] shrink-0 size-[24px]"
                  data-node-id="342:1617"
                  data-name="Layout / Free Credits For Your First Reading"
                >
                  <div
                    className="relative shrink-0 size-[12px]"
                    data-node-id="342:1811"
                    data-name="Container / Check"
                  >
                    <img
                      alt=""
                      className="absolute block inset-0 max-w-none size-full"
                      src={imgContainerCheck}
                    />
                  </div>
                </div>
                <p
                  className="[word-break:break-word] flex-[1_0_0] font-serif font-normal leading-[normal] min-w-px relative text-[18px] text-black"
                  data-node-id="342:1619"
                >
                  {text("Clear prices shown before every reading ")}
                </p>
              </div>
              <div
                className="content-stretch flex gap-[12px] items-center relative shrink-0 w-full"
                data-node-id="342:1620"
                data-name="Content Group / A directory to find an advisor who suits you"
              >
                <div
                  className="bg-[var(--sage)] border border-black border-solid content-stretch flex items-center justify-center relative rounded-[999px] shrink-0 size-[24px]"
                  data-node-id="342:1621"
                  data-name="Layout / An Effective Psychic Client Matching Tool"
                >
                  <div
                    className="relative shrink-0 size-[12px]"
                    data-node-id="342:1814"
                    data-name="Container / Check"
                  >
                    <img
                      alt=""
                      className="absolute block inset-0 max-w-none size-full"
                      src={imgContainerCheck}
                    />
                  </div>
                </div>
                <p
                  className="[word-break:break-word] flex-[1_0_0] font-serif font-normal leading-[normal] min-w-px relative text-[18px] text-black"
                  data-node-id="342:1623"
                >
                  {text("A directory to find an advisor who suits you ")}
                </p>
              </div>
            </div>
          </div>
        </section>
      </div>
      <ContentLink
        className="border-b border-black border-solid border-t content-stretch flex flex-col gap-[15px] h-[411px] items-center justify-center overflow-clip px-[120px] py-[64px] relative shrink-0 w-full"
        data-node-id="342:1624"
        data-name="Button / NOT SURE WHERE IS B…"
        label="NOT SURE WHERE TO BEGIN? Start First Reading Try free minutes of chat and see for yourself"
      >
        <div aria-hidden className="absolute inset-0 pointer-events-none">
          <div className="absolute bg-[var(--cream)] inset-0" />
          <img
            alt=""
            className="absolute max-w-none mix-blend-overlay object-cover opacity-62 size-full"
            src={imgMediaReviews}
          />
        </div>
        <div
          className="[word-break:break-word] font-display leading-[0] not-italic relative shrink-0 text-[#2f241c] text-[36px] text-center tracking-[1.44px] whitespace-nowrap"
          data-node-id="342:1625"
        >
          <p className="leading-[normal] mb-0">
            {text("NOT SURE WHERE ")}
          </p>
          <p className="leading-[normal]">
            {text("TO BEGIN?")}
          </p>
        </div>
        <div
          className="content-stretch flex flex-col h-[77px] items-center justify-between relative shrink-0 w-[852px]"
          data-node-id="377:1500"
          data-name="Layout / NOT SURE WHERE IS B…"
        >
          <span
            className="border-[0.5px] border-black border-solid content-stretch cursor-pointer flex flex-col h-[55px] items-center justify-center overflow-clip px-[219px] py-[12px] relative rounded-[8px] shrink-0 w-[439px]"
            data-node-id="342:1627"
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
                src={imgMediaReviews}
              />
            </div>
            <p
              className="[word-break:break-word] font-display leading-[normal] not-italic relative shrink-0 text-[24px] text-black text-center tracking-[1.2px] whitespace-nowrap"
              data-node-id="I342:1627;113:303"
            >
              {text("Start First Reading ")}
            </p>
          </span>
          {freeMinutes > 0 ? (
            <p
              className="[word-break:break-word] font-serif font-normal leading-[0] relative shrink-0 text-[#2f241c] text-[22px] text-center w-[540.545px]"
              data-node-id="342:1626"
              data-testid="reviews-free-minutes"
            >
              <span className="leading-[normal]">{text(`Try `)}</span>
              <span className="font-serif font-bold leading-[normal] text-[#c29a3d]">
                {text("{value0} free minutes ", { value0: freeMinutes })}
              </span>
              <span className="leading-[normal]">
                {text(` of chat and see for yourself`)}
              </span>
            </p>
          ) : null}
        </div>
      </ContentLink>
      <div
        className="bg-[#e0d5c3] content-stretch flex flex-col h-[1138px] items-center overflow-clip relative shrink-0 w-full"
        data-node-id="788:847"
        data-name="Container / Services & Feature Highlights"
      >
        <section
          className="content-stretch flex flex-col gap-[40px] h-[440.28px] items-center justify-center overflow-clip pb-[60px] pt-[80px] px-[160px] relative shrink-0 w-[1440px] figma-section"
          data-node-id="342:1629"
          data-name="Section / Free Services"
        >
          <section
            className="content-stretch flex gap-[16px] items-center justify-center relative shrink-0 figma-section"
            data-node-id="342:1630"
            data-name="Section / Header"
          >
            <div
              className="relative shrink-0 size-[34.07px]"
              data-node-id="342:1631"
              data-name="Instance / Star"
            />
            <p
              className="[word-break:break-word] font-display leading-[normal] not-italic relative shrink-0 text-[#2f241c] text-[32px] text-center tracking-[1.28px] whitespace-nowrap"
              data-node-id="342:1633"
            >
              {text("Free Tools ")}
            </p>
            <div
              className="relative shrink-0 size-[34.07px]"
              data-node-id="342:1634"
              data-name="Instance / Star / 02"
            />
          </section>
          <div
            className="content-stretch flex items-start relative shrink-0 w-full"
            data-node-id="342:1636"
            data-name="Layout / Free Services"
          >
            <div
              className="content-stretch flex flex-[1_0_0] flex-col gap-[20px] items-start min-w-px mr-[-200px] relative"
              data-node-id="342:1637"
              data-name="Layout / Free Services"
            >
              <p
                className="[word-break:break-word] font-serif font-medium leading-[normal] relative shrink-0 text-[#2f241c] text-[22px] whitespace-nowrap"
                data-node-id="342:1638"
              >
                {text("Staryield offers a range of free tools: ")}
              </p>
              <div
                className="content-stretch flex flex-col gap-[12px] items-start relative shrink-0"
                data-node-id="342:1639"
                data-name="Content Group / Natal chart calculator"
              >
                <div
                  className="content-stretch flex gap-[12px] items-center relative shrink-0"
                  data-node-id="342:1640"
                  data-name="Control / Natal chart calculator"
                >
                  <div
                    className="relative shrink-0 size-[34.07px]"
                    data-node-id="342:1641"
                    data-name="Instance / Star"
                  >
                    <img
                      alt=""
                      className="absolute block inset-0 max-w-none size-full"
                      height="34.07"
                      src={imgInstanceStar}
                      width="34.07"
                    />
                  </div>
                  <p
                    className="[word-break:break-word] font-serif font-normal leading-[normal] relative shrink-0 text-[#2f241c] text-[18px] whitespace-nowrap"
                    data-node-id="342:1643"
                  >
                    {text("Natal chart calculator ")}
                  </p>
                </div>
                <div
                  className="content-stretch flex gap-[12px] items-center relative shrink-0"
                  data-node-id="342:1644"
                  data-name="Control / Life Path number calculator"
                >
                  <div
                    className="relative shrink-0 size-[34.07px]"
                    data-node-id="342:1645"
                    data-name="Instance / Star"
                  >
                    <img
                      alt=""
                      className="absolute block inset-0 max-w-none size-full"
                      height="34.07"
                      src={imgInstanceStar}
                      width="34.07"
                    />
                  </div>
                  <p
                    className="[word-break:break-word] font-serif font-normal leading-[normal] relative shrink-0 text-[#2f241c] text-[18px] whitespace-nowrap"
                    data-node-id="342:1647"
                  >
                    {text("Life Path number calculator ")}
                  </p>
                </div>
                <div
                  className="content-stretch flex gap-[12px] items-center relative shrink-0"
                  data-node-id="342:1648"
                  data-name="Control / Astrology calculator"
                >
                  <div
                    className="relative shrink-0 size-[34.07px]"
                    data-node-id="342:1649"
                    data-name="Instance / Star"
                  >
                    <img
                      alt=""
                      className="absolute block inset-0 max-w-none size-full"
                      height="34.07"
                      src={imgInstanceStar}
                      width="34.07"
                    />
                  </div>
                  <p
                    className="[word-break:break-word] font-serif font-normal leading-[normal] relative shrink-0 text-[#2f241c] text-[18px] whitespace-nowrap"
                    data-node-id="342:1651"
                  >
                    {text("Astrology calculator ")}
                  </p>
                </div>
                <div
                  className="content-stretch flex gap-[12px] items-center relative shrink-0"
                  data-node-id="342:1652"
                  data-name="Control / Personal daily horoscopes"
                >
                  <div
                    className="relative shrink-0 size-[34.07px]"
                    data-node-id="342:1653"
                    data-name="Instance / Star"
                  >
                    <img
                      alt=""
                      className="absolute block inset-0 max-w-none size-full"
                      height="34.07"
                      src={imgInstanceStar}
                      width="34.07"
                    />
                  </div>
                  <p
                    className="[word-break:break-word] font-serif font-normal leading-[normal] relative shrink-0 text-[#2f241c] text-[18px] whitespace-nowrap"
                    data-node-id="342:1655"
                  >
                    {text("Personal daily horoscopes ")}
                  </p>
                </div>
              </div>
            </div>
            <div
              className="[word-break:break-word] content-stretch flex flex-[1_0_0] flex-col font-serif font-light gap-[16px] items-start leading-[1.6] min-w-px relative text-[#2f241c] text-[16px]"
              data-node-id="342:1656"
              data-name="Layout / Free Services / 02"
            >
              <p className="relative shrink-0 w-full" data-node-id="342:1657">
                {text(
                  "Each calculator comes with an article that explains how it works and where it comes from. ",
                )}
              </p>
              <p className="relative shrink-0 w-full" data-node-id="342:1658">
                {text(
                  "Staryield works in your browser on your phone, tablet or computer. Explore at your own pace. ",
                )}
              </p>
            </div>
          </div>
        </section>
        <section
          className="content-stretch flex flex-col h-[698px] items-center justify-center overflow-clip px-[160px] py-[80px] relative shrink-0 w-[1440px] figma-section"
          data-node-id="342:1659"
          data-name="Section / Feature Highlights"
        >
          <div
            className="[word-break:break-word] content-stretch flex flex-col gap-[32px] items-start relative shrink-0 text-black w-full whitespace-nowrap"
            data-node-id="342:1660"
            data-name="Container / Features Grid"
          >
            <div
              className="content-stretch flex flex-col gap-[12px] items-start relative shrink-0 w-full"
              data-node-id="342:1661"
              data-name="Layout / Features Grid"
            >
              <p
                className="font-serif font-normal leading-[normal] relative shrink-0 text-[20px]"
                data-node-id="342:1662"
              >
                {text("Intuitive Staryield Advisors ")}
              </p>
              <p
                className="font-serif font-light leading-[1.5] relative shrink-0 text-[16px]"
                data-node-id="342:1663"
              >
                {text(
                  "Advisors with many different specialties, from tarot and astrology to numerology and mediumship. ",
                )}
              </p>
            </div>
            <div
              className="content-stretch flex flex-col gap-[12px] items-start relative shrink-0 w-full"
              data-node-id="342:1664"
              data-name="Layout / Features Grid / 02"
            >
              <p
                className="font-serif font-normal leading-[normal] relative shrink-0 text-[20px]"
                data-node-id="342:1665"
              >
                {text("A Simple Experience ")}
              </p>
              <p
                className="font-serif font-light leading-[1.5] relative shrink-0 text-[16px]"
                data-node-id="342:1666"
              >
                {text(
                  "Clear pages, readable prices and a chat that works on any screen. ",
                )}
              </p>
            </div>
            <div
              className="content-stretch flex flex-col gap-[12px] items-start relative shrink-0 w-full"
              data-node-id="342:1667"
              data-name="Layout / Features Grid / 03"
            >
              <p
                className="font-serif font-normal leading-[normal] relative shrink-0 text-[20px]"
                data-node-id="342:1668"
              >
                {text("Many Reading Themes ")}
              </p>
              <p
                className="font-serif font-light leading-[1.5] relative shrink-0 text-[16px]"
                data-node-id="342:1669"
              >
                {text(
                  "Readings about love, career and life choices, with tarot, aura readings, astrology, numerology and more, for entertainment purposes only. ",
                )}
              </p>
            </div>
            <div
              className="content-stretch flex flex-col gap-[12px] items-start relative shrink-0 w-full"
              data-node-id="342:1670"
              data-name="Layout / Features Grid / 04"
            >
              <p
                className="font-serif font-normal leading-[normal] relative shrink-0 text-[20px]"
                data-node-id="342:1671"
              >
                {text(`Welcome Offers`)}
              </p>
              <p
                className="font-serif font-light leading-[1.5] relative shrink-0 text-[16px]"
                data-node-id="342:1672"
              >
                {text(
                  "New accounts may receive free chat minutes when they sign up. Offers can change: the current one is always shown on the pricing page. ",
                )}
              </p>
            </div>
            <div
              className="content-stretch flex flex-col gap-[12px] items-start relative shrink-0 w-full"
              data-node-id="342:1673"
              data-name="Layout / Features Grid / 05"
            >
              <p
                className="font-serif font-normal leading-[normal] relative shrink-0 text-[20px]"
                data-node-id="342:1674"
              >
                {text("Live Chat ")}
              </p>
              <p
                className="font-serif font-light leading-[1.5] relative shrink-0 text-[16px]"
                data-node-id="342:1675"
              >
                {text(
                  "Connect on the go with real-time text readings. For a slower pace, you can also order a custom reading from an advisor. ",
                )}
              </p>
            </div>
            <div
              className="content-stretch flex flex-col gap-[12px] items-start relative shrink-0 w-full"
              data-node-id="342:1676"
              data-name="Layout / Features Grid / 06"
            >
              <p
                className="font-serif font-normal leading-[normal] relative shrink-0 text-[20px]"
                data-node-id="342:1677"
              >
                {text("Find an Advisor ")}
              </p>
              <p
                className="font-serif font-light leading-[1.5] relative shrink-0 text-[16px]"
                data-node-id="342:1678"
              >
                {text(
                  "Browse the directory, compare specialties and the ratings left by other clients, and choose the advisor who feels right. ",
                )}
              </p>
            </div>
          </div>
        </section>
      </div>
      <section
        className="bg-[var(--warm-gray)] border-b border-black border-solid content-stretch flex flex-col gap-[40px] h-[426px] items-center justify-center overflow-clip px-[160px] py-[80px] relative shrink-0 w-full figma-section"
        data-node-id="342:1679"
        data-name="Section / About The Company"
      >
        <section
          className="content-stretch flex gap-[16px] items-center justify-center relative shrink-0 figma-section"
          data-node-id="342:1680"
          data-name="Section / Header"
        >
          <div
            className="relative shrink-0 size-[34.07px]"
            data-node-id="342:1681"
            data-name="Instance / Star"
          />
          <p
            className="[word-break:break-word] font-display leading-[normal] not-italic relative shrink-0 text-[#2f241c] text-[48px] text-center tracking-[1.92px] whitespace-nowrap"
            data-node-id="342:1683"
          >
            {text("About the Company ")}
          </p>
          <div
            className="relative shrink-0 size-[34.07px]"
            data-node-id="342:1684"
            data-name="Instance / Star / 02"
          />
        </section>
        <div
          className="[word-break:break-word] content-stretch flex flex-col gap-[20px] items-start leading-[1.6] relative shrink-0 text-[#2f241c] w-full"
          data-node-id="342:1686"
          data-name="Layout / About The Company"
        >
          <p
            className="font-serif font-normal relative shrink-0 text-[32px] w-full"
            data-node-id="342:1688"
          >
            {text(
              "Staryield is operated by {value0}, {value1}. ",
              { value0: COMPANY.legalName, value1: COMPANY.addressLine },
            )}
          </p>
          <p
            className="font-serif font-light relative shrink-0 text-[18px] w-full"
            data-node-id="342:1689"
          >
            {text(
              "Readings are offered for entertainment purposes only, and we do not certify or verify advisors' abilities. You can write to us at {value0}. Reviews of Staryield are published by independent users on Trustpilot. ",
              { value0: COMPANY.supportEmail },
            )}
          </p>
        </div>
      </section>
      <section
        className="bg-[var(--sage)] content-stretch flex flex-col gap-[40px] items-center overflow-clip pb-[80px] pt-[60px] px-[160px] relative shrink-0 w-full figma-section"
        data-node-id="342:1716"
        data-name="Section / Types Of Readings"
      >
        <section
          className="content-stretch flex gap-[16px] items-center justify-center relative shrink-0 w-full figma-section"
          data-node-id="342:1717"
          data-name="Section / Header"
        >
          <div
            className="relative shrink-0 size-[34.07px]"
            data-node-id="342:1718"
            data-name="Instance / Star"
          >
            <img
              alt=""
              className="absolute block inset-0 max-w-none size-full"
              height="34.07"
              src={imgInstanceStar}
              width="34.07"
            />
          </div>
          <p
            className="[word-break:break-word] font-display leading-[normal] not-italic relative shrink-0 text-[32px] text-center text-white tracking-[1.28px] whitespace-nowrap"
            data-node-id="342:1720"
          >
            {text("Reading Types ")}
          </p>
          <div
            className="relative shrink-0 size-[34.07px]"
            data-node-id="342:1721"
            data-name="Instance / Star / 02"
          >
            <img
              alt=""
              className="absolute block inset-0 max-w-none size-full"
              height="34.07"
              src={imgInstanceStar}
              width="34.07"
            />
          </div>
        </section>
        <div
          className="content-stretch flex items-center relative shrink-0 w-full"
          data-node-id="342:1723"
          data-name="Layout / Types Of Readings"
        >
          <div
            className="[word-break:break-word] content-stretch flex flex-[1_0_0] flex-col gap-[20px] items-start min-w-px relative text-[18px] text-white whitespace-nowrap"
            data-node-id="342:1724"
            data-name="Layout / Types Of Readings"
          >
            <p
              className="font-serif font-medium leading-[1.6] relative shrink-0"
              data-node-id="342:1725"
            >
              {text(
                `Explore paths tailored to you. Our advisors offer readings such as:`,
              )}
            </p>
            <div
              className="content-stretch flex flex-col font-serif font-light gap-[16px] items-start leading-[normal] relative shrink-0"
              data-node-id="342:1726"
              data-name="Layout / Types Of Readings"
            >
              <p className="relative shrink-0" data-node-id="342:1727">
                {text(
                  ` Tarot: reflect on the road ahead and the choices in front of you.`,
                )}
              </p>
              <p className="relative shrink-0" data-node-id="342:1728">
                {text(
                  ` Psychic readings: a warm, personal conversation with an intuitive advisor.`,
                )}
              </p>
              <p className="relative shrink-0" data-node-id="342:1729">
                {text(
                  ` Astrology: explore your natal chart and the planetary transits of the moment.`,
                )}
              </p>
            </div>
          </div>
        </div>
        <div
          className="h-0 relative shrink-0 w-full"
          data-node-id="342:1730"
          data-name="Divider / Horizontal"
        >
          <div className="absolute inset-[-2px_0_0_0]">
            <img
              alt=""
              className="block max-w-none size-full"
              src={imgDividerHorizontal1}
            />
          </div>
        </div>
        <div
          className="content-stretch flex flex-col gap-[16px] items-center relative shrink-0 w-[1170px]"
          data-node-id="342:1731"
          data-name="Layout / Types Of Readings / 02"
        >
          <p
            className="[word-break:break-word] font-display leading-[normal] not-italic relative shrink-0 text-[32px] text-white whitespace-nowrap"
            data-node-id="342:1732"
          >
            {text("Other Available Modalities: ")}
          </p>
          <div
            className="content-start flex flex-wrap gap-[12px] items-start justify-center relative shrink-0 w-full"
            data-node-id="342:1733"
            data-name="Layout / 02"
          >
            <div
              className="bg-[#e0d5c3] border border-[rgba(21,16,11,0.64)] border-solid content-stretch flex items-start px-[16px] py-[6px] relative rounded-[4px] shrink-0"
              data-node-id="342:1734"
              data-name="Control / Numerology"
            >
              <p
                className="[word-break:break-word] font-serif font-normal leading-[normal] relative shrink-0 text-[#2f241c] text-[14px] whitespace-nowrap"
                data-node-id="342:1735"
              >
                {text("Numerology ")}
              </p>
            </div>
            <div
              className="bg-[#e0d5c3] border border-[rgba(21,16,11,0.64)] border-solid content-stretch flex items-start px-[16px] py-[6px] relative rounded-[4px] shrink-0"
              data-node-id="342:1736"
              data-name="Control / Clairvoyance"
            >
              <p
                className="[word-break:break-word] font-serif font-normal leading-[normal] relative shrink-0 text-[#2f241c] text-[14px] whitespace-nowrap"
                data-node-id="342:1737"
              >
                {text("Clairvoyance ")}
              </p>
            </div>
            <div
              className="bg-[#e0d5c3] border border-[rgba(21,16,11,0.64)] border-solid content-stretch flex items-start px-[16px] py-[6px] relative rounded-[4px] shrink-0"
              data-node-id="342:1738"
              data-name="Control / Occult Reading"
            >
              <p
                className="[word-break:break-word] font-serif font-normal leading-[normal] relative shrink-0 text-[#2f241c] text-[14px] whitespace-nowrap"
                data-node-id="342:1739"
              >
                {text("Occult Reading ")}
              </p>
            </div>
            <div
              className="bg-[#e0d5c3] border border-[rgba(21,16,11,0.64)] border-solid content-stretch flex items-start px-[16px] py-[6px] relative rounded-[4px] shrink-0"
              data-node-id="342:1740"
              data-name="Control / Angel Reading"
            >
              <p
                className="[word-break:break-word] font-serif font-normal leading-[normal] relative shrink-0 text-[#2f241c] text-[14px] whitespace-nowrap"
                data-node-id="342:1741"
              >
                {text("Angel Reading ")}
              </p>
            </div>
            <div
              className="bg-[#e0d5c3] border border-[rgba(21,16,11,0.64)] border-solid content-stretch flex items-start px-[16px] py-[6px] relative rounded-[4px] shrink-0"
              data-node-id="342:1742"
              data-name="Control / Spirituality"
            >
              <p
                className="[word-break:break-word] font-serif font-normal leading-[normal] relative shrink-0 text-[#2f241c] text-[14px] whitespace-nowrap"
                data-node-id="342:1743"
              >
                {text("Spirituality ")}
              </p>
            </div>
            <div
              className="bg-[#e0d5c3] border border-[rgba(21,16,11,0.64)] border-solid content-stretch flex items-start px-[16px] py-[6px] relative rounded-[4px] shrink-0"
              data-node-id="342:1744"
              data-name="Control / Mediumship"
            >
              <p
                className="[word-break:break-word] font-serif font-normal leading-[normal] relative shrink-0 text-[#2f241c] text-[14px] whitespace-nowrap"
                data-node-id="342:1745"
              >
                {text("Mediumship ")}
              </p>
            </div>
            <div
              className="bg-[#e0d5c3] border border-[rgba(21,16,11,0.64)] border-solid content-stretch flex items-start px-[16px] py-[6px] relative rounded-[4px] shrink-0"
              data-node-id="342:1746"
              data-name="Control / Runes"
            >
              <p
                className="[word-break:break-word] font-serif font-normal leading-[normal] relative shrink-0 text-[#2f241c] text-[14px] whitespace-nowrap"
                data-node-id="342:1747"
              >
                {text("Runes ")}
              </p>
            </div>
            <div
              className="bg-[#e0d5c3] border border-[rgba(21,16,11,0.64)] border-solid content-stretch flex items-start px-[16px] py-[6px] relative rounded-[4px] shrink-0"
              data-node-id="342:1748"
              data-name="Control / Pendulum"
            >
              <p
                className="[word-break:break-word] font-serif font-normal leading-[normal] relative shrink-0 text-[#2f241c] text-[14px] whitespace-nowrap"
                data-node-id="342:1749"
              >
                {text("Pendulum ")}
              </p>
            </div>
            <div
              className="bg-[#e0d5c3] border border-[rgba(21,16,11,0.64)] border-solid content-stretch flex items-start px-[16px] py-[6px] relative rounded-[4px] shrink-0"
              data-node-id="342:1750"
              data-name="Control / Past Life"
            >
              <p
                className="[word-break:break-word] font-serif font-normal leading-[normal] relative shrink-0 text-[#2f241c] text-[14px] whitespace-nowrap"
                data-node-id="342:1751"
              >
                {text("Past Life ")}
              </p>
            </div>
            <div
              className="bg-[#e0d5c3] border border-[rgba(21,16,11,0.64)] border-solid content-stretch flex items-start px-[16px] py-[6px] relative rounded-[4px] shrink-0"
              data-node-id="342:1752"
              data-name="Control / Aura"
            >
              <p
                className="[word-break:break-word] font-serif font-normal leading-[normal] relative shrink-0 text-[#2f241c] text-[14px] whitespace-nowrap"
                data-node-id="342:1753"
              >
                {text("Aura ")}
              </p>
            </div>
          </div>
        </div>
      </section>
    </div>
  );
}

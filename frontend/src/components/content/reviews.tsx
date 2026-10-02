"use client";
import { useI18n } from "@/i18n/i18n-provider";
/* Figma 342:1455 — reusable responsive content regions. */
/* eslint-disable @next/next/no-img-element */
import { ContentLink } from "@/components/content/content-link";
const assetPathPrefix = "https://assets.staryield.net/assets/figma";
const imgMediaReviews = `${assetPathPrefix}/fb9ee.jpg`;
const imgInstanceStar = `${assetPathPrefix}/e7e99.png`;
const imgInstanceStar05 = `${assetPathPrefix}/1c3f4.png`;
const imgDividerHorizontal = `${assetPathPrefix}/2e925.svg`;
const imgContainerCheck = `${assetPathPrefix}/ceeda.svg`;
const imgDividerHorizontal1 = `${assetPathPrefix}/5455a.svg`;
export default function ScreenDesktopReviews() {
  const { text } = useI18n();
  return (
    <div
      className="figma-content flex w-full flex-col items-center bg-cream"
      data-node-id="342:1455"
      data-name="Screen / Desktop / Reviews"
    >
      <div
        className="bg-[var(--cream)] border-[var(--cream)] border-b border-solid content-stretch flex flex-col gap-[16px] h-[252px] items-center overflow-clip pb-[80px] pt-[45px] px-[120px] relative shrink-0 w-full"
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
          className="content-stretch flex flex-col gap-[6px] items-center justify-center relative shrink-0"
          data-node-id="378:1501"
          data-name="Content Group / 17,419 reviews"
        >
          <div
            className="content-stretch flex flex-col gap-[8px] items-center relative shrink-0"
            data-node-id="342:1481"
            data-name="Control / 17,419 reviews"
          >
            <p
              className="[word-break:break-word] font-serif font-normal leading-[normal] relative shrink-0 text-[20px] text-black whitespace-nowrap"
              data-node-id="342:1482"
            >
              {text("17,419 reviews ")}
            </p>
            <div
              className="content-stretch flex gap-[12px] items-center relative shrink-0"
              data-node-id="342:1483"
              data-name="Control / | 4.4 rating"
            >
              <div
                className="content-stretch flex gap-[4px] items-center relative shrink-0"
                data-node-id="342:1484"
                data-name="Layout / | 4.4 Rating"
              >
                <div
                  className="relative shrink-0 size-[34.07px]"
                  data-node-id="342:1485"
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
                <div
                  className="relative shrink-0 size-[34.07px]"
                  data-node-id="342:1487"
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
                <div
                  className="relative shrink-0 size-[34.07px]"
                  data-node-id="342:1489"
                  data-name="Instance / Star / 03"
                >
                  <img
                    alt=""
                    className="absolute block inset-0 max-w-none size-full"
                    height="34.07"
                    src={imgInstanceStar}
                    width="34.07"
                  />
                </div>
                <div
                  className="relative shrink-0 size-[34.07px]"
                  data-node-id="342:1491"
                  data-name="Instance / Star / 04"
                >
                  <img
                    alt=""
                    className="absolute block inset-0 max-w-none size-full"
                    height="34.07"
                    src={imgInstanceStar}
                    width="34.07"
                  />
                </div>
                <div
                  className="relative shrink-0 size-[34.07px]"
                  data-node-id="342:1493"
                  data-name="Instance / Star / 05"
                >
                  <img
                    alt=""
                    className="absolute block inset-0 max-w-none size-full"
                    height="34.07"
                    src={imgInstanceStar05}
                    width="34.07"
                  />
                </div>
              </div>
              <p
                className="[word-break:break-word] font-serif font-normal leading-[normal] relative shrink-0 text-[22px] text-black whitespace-nowrap"
                data-node-id="342:1495"
              >
                {text("| 4.4 rating ")}
              </p>
            </div>
          </div>
          <p
            className="[word-break:break-word] font-serif font-light leading-[normal] relative shrink-0 text-[24px] text-black text-center tracking-[0.72px] whitespace-nowrap"
            data-node-id="342:1496"
          >
            {text("Reviews from our customers ")}
          </p>
        </div>
      </div>
      <section
        className="bg-gradient-to-b content-stretch flex flex-col from-[36.538%] from-[var(--cream)] items-start overflow-clip pb-[80px] pt-[40px] px-[120px] relative shrink-0 to-[var(--sage)] w-full figma-section"
        data-node-id="342:1497"
        data-name="Section / Customer Review Cards"
      >
        <div
          className="content-stretch flex flex-col gap-[24px] items-start relative shrink-0 w-full"
          data-node-id="342:1498"
          data-name="Container / Review Grid"
        >
          <div
            className="border-[0.5px] border-black border-solid content-stretch flex flex-col gap-[16px] items-start p-[24px] relative rounded-[8px] shrink-0 w-full"
            data-node-id="342:1499"
            data-name="Card / Review"
          >
            <div
              aria-hidden
              className="absolute inset-0 pointer-events-none rounded-[8px]"
            >
              <div className="absolute bg-[var(--warm-gray)] inset-0 rounded-[8px]" />
              <img
                alt=""
                className="absolute max-w-none mix-blend-overlay object-cover rounded-[8px] size-full"
                src={imgMediaReviews}
              />
            </div>
            <div
              className="content-stretch flex items-center justify-between relative shrink-0 w-full"
              data-node-id="342:1500"
              data-name="Layout / Review"
            >
              <div
                className="[word-break:break-word] content-stretch flex flex-col gap-px items-start leading-[normal] relative shrink-0 text-black whitespace-nowrap"
                data-node-id="342:1501"
                data-name="Control / Thayna"
              >
                <p
                  className="font-serif font-semibold relative shrink-0 text-[20px]"
                  data-node-id="342:1502"
                >
                  {text("Thayna ")}
                </p>
                <p
                  className="font-serif font-extralight relative shrink-0 text-[14px]"
                  data-node-id="342:1503"
                >
                  {text("24 Aug, 2026 ")}
                </p>
              </div>
              <div
                className="content-stretch flex gap-[4px] items-center relative shrink-0"
                data-node-id="342:1504"
                data-name="Layout / Review"
              >
                <div
                  className="relative shrink-0 size-[34.07px]"
                  data-node-id="342:1505"
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
                <div
                  className="relative shrink-0 size-[34.07px]"
                  data-node-id="342:1507"
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
                <div
                  className="relative shrink-0 size-[34.07px]"
                  data-node-id="342:1509"
                  data-name="Instance / Star / 03"
                >
                  <img
                    alt=""
                    className="absolute block inset-0 max-w-none size-full"
                    height="34.07"
                    src={imgInstanceStar}
                    width="34.07"
                  />
                </div>
                <div
                  className="relative shrink-0 size-[34.07px]"
                  data-node-id="342:1511"
                  data-name="Instance / Star / 04"
                >
                  <img
                    alt=""
                    className="absolute block inset-0 max-w-none size-full"
                    height="34.07"
                    src={imgInstanceStar}
                    width="34.07"
                  />
                </div>
                <div
                  className="relative shrink-0 size-[34.07px]"
                  data-node-id="342:1513"
                  data-name="Instance / Star / 05"
                >
                  <img
                    alt=""
                    className="absolute block inset-0 max-w-none size-full"
                    height="34.07"
                    src={imgInstanceStar}
                    width="34.07"
                  />
                </div>
              </div>
            </div>
            <p
              className="[word-break:break-word] font-serif font-normal leading-[1.5] relative shrink-0 text-[16px] text-black w-full"
              data-node-id="342:1515"
            >
              {text(
                "She was on point from the beginning! Could translate exactly how I felt! Fast reply and lots of details!! Thank you! ",
              )}
            </p>
          </div>
          <div
            className="border-[0.5px] border-black border-solid content-stretch flex flex-col gap-[16px] items-start p-[24px] relative rounded-[8px] shrink-0 w-full"
            data-node-id="342:1516"
            data-name="Card / Review / 02"
          >
            <div
              aria-hidden
              className="absolute inset-0 pointer-events-none rounded-[8px]"
            >
              <div className="absolute bg-[var(--warm-gray)] inset-0 rounded-[8px]" />
              <img
                alt=""
                className="absolute max-w-none mix-blend-overlay object-cover rounded-[8px] size-full"
                src={imgMediaReviews}
              />
            </div>
            <div
              className="content-stretch flex items-center justify-between relative shrink-0 w-full"
              data-node-id="342:1517"
              data-name="Layout / 02"
            >
              <div
                className="[word-break:break-word] content-stretch flex flex-col gap-px items-start leading-[normal] relative shrink-0 text-black whitespace-nowrap"
                data-node-id="342:1518"
                data-name="Control / Surinder"
              >
                <p
                  className="font-serif font-semibold relative shrink-0 text-[20px]"
                  data-node-id="342:1519"
                >
                  {text("Surinder ")}
                </p>
                <p
                  className="font-serif font-extralight relative shrink-0 text-[14px]"
                  data-node-id="342:1520"
                >
                  {text("24 Aug, 2026 ")}
                </p>
              </div>
              <div
                className="content-stretch flex gap-[4px] items-center relative shrink-0"
                data-node-id="342:1521"
                data-name="Layout / 02"
              >
                <div
                  className="relative shrink-0 size-[34.07px]"
                  data-node-id="342:1522"
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
                <div
                  className="relative shrink-0 size-[34.07px]"
                  data-node-id="342:1524"
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
                <div
                  className="relative shrink-0 size-[34.07px]"
                  data-node-id="342:1526"
                  data-name="Instance / Star / 03"
                >
                  <img
                    alt=""
                    className="absolute block inset-0 max-w-none size-full"
                    height="34.07"
                    src={imgInstanceStar}
                    width="34.07"
                  />
                </div>
                <div
                  className="relative shrink-0 size-[34.07px]"
                  data-node-id="342:1528"
                  data-name="Instance / Star / 04"
                >
                  <img
                    alt=""
                    className="absolute block inset-0 max-w-none size-full"
                    height="34.07"
                    src={imgInstanceStar}
                    width="34.07"
                  />
                </div>
                <div
                  className="relative shrink-0 size-[34.07px]"
                  data-node-id="342:1530"
                  data-name="Instance / Star / 05"
                >
                  <img
                    alt=""
                    className="absolute block inset-0 max-w-none size-full"
                    height="34.07"
                    src={imgInstanceStar}
                    width="34.07"
                  />
                </div>
              </div>
            </div>
            <p
              className="[word-break:break-word] font-serif font-normal leading-[1.5] relative shrink-0 text-[16px] text-black w-full"
              data-node-id="342:1532"
            >
              {text(
                "Can u pls unhide my answer the reading is great and accurate thank alot for your reading ",
              )}
            </p>
          </div>
          <div
            className="border-[0.5px] border-black border-solid content-stretch flex flex-col gap-[16px] items-start p-[24px] relative rounded-[8px] shrink-0 w-full"
            data-node-id="342:1533"
            data-name="Card / Review / 03"
          >
            <div
              aria-hidden
              className="absolute inset-0 pointer-events-none rounded-[8px]"
            >
              <div className="absolute bg-[var(--warm-gray)] inset-0 rounded-[8px]" />
              <img
                alt=""
                className="absolute max-w-none mix-blend-overlay object-cover rounded-[8px] size-full"
                src={imgMediaReviews}
              />
            </div>
            <div
              className="content-stretch flex items-center justify-between relative shrink-0 w-full"
              data-node-id="342:1534"
              data-name="Layout / 03"
            >
              <div
                className="[word-break:break-word] content-stretch flex flex-col gap-px items-start leading-[normal] relative shrink-0 text-black whitespace-nowrap"
                data-node-id="342:1535"
                data-name="Control / Mariana"
              >
                <p
                  className="font-serif font-semibold relative shrink-0 text-[20px]"
                  data-node-id="342:1536"
                >
                  {text("Mariana ")}
                </p>
                <p
                  className="font-serif font-extralight relative shrink-0 text-[14px]"
                  data-node-id="342:1537"
                >
                  {text("23 Aug, 2026 ")}
                </p>
              </div>
              <div
                className="content-stretch flex gap-[4px] items-center relative shrink-0"
                data-node-id="342:1538"
                data-name="Layout / 03"
              >
                <div
                  className="relative shrink-0 size-[34.07px]"
                  data-node-id="342:1539"
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
                <div
                  className="relative shrink-0 size-[34.07px]"
                  data-node-id="342:1541"
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
                <div
                  className="relative shrink-0 size-[34.07px]"
                  data-node-id="342:1543"
                  data-name="Instance / Star / 03"
                >
                  <img
                    alt=""
                    className="absolute block inset-0 max-w-none size-full"
                    height="34.07"
                    src={imgInstanceStar}
                    width="34.07"
                  />
                </div>
                <div
                  className="relative shrink-0 size-[34.07px]"
                  data-node-id="342:1545"
                  data-name="Instance / Star / 04"
                >
                  <img
                    alt=""
                    className="absolute block inset-0 max-w-none size-full"
                    height="34.07"
                    src={imgInstanceStar}
                    width="34.07"
                  />
                </div>
                <div
                  className="relative shrink-0 size-[34.07px]"
                  data-node-id="342:1547"
                  data-name="Instance / Star / 05"
                >
                  <img
                    alt=""
                    className="absolute block inset-0 max-w-none size-full"
                    height="34.07"
                    src={imgInstanceStar}
                    width="34.07"
                  />
                </div>
              </div>
            </div>
            <p
              className="[word-break:break-word] font-serif font-normal leading-[1.5] relative shrink-0 text-[16px] text-black w-full"
              data-node-id="342:1549"
            >
              {text(
                "Incredible session, felt truly seen and understood. The guidance was specific and immediately helpful. ",
              )}
            </p>
          </div>
        </div>
      </section>
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
              "What if there was a trusted space where you could explore fresh perspectives on your past, present, and future? ",
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
              "With over 1000 gifted psychics, our users receive guidance 24/7, helping them better understand themselves and the people around them. ",
            )}
          </p>
          <p
            className="font-serif font-normal leading-[1.6] relative shrink-0 text-[20px] w-full"
            data-node-id="342:1563"
          >
            {text(
              "If you are looking for compassionate, nonjudgmental advice and want to learn more about yourself and others, Staryield is the place to be. Our platform connects you with trusted advisors. Use our search filters or take the matching quiz to find the perfect guide for your journey. ",
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
              {text("Facts About Staryield ")}
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
                "If you are wondering whether Staryield is the right choice for you, here are some interesting facts about our website: ",
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
                  `• Many of our reviews don't just focus on the psychics' work; some highlight our customer service. Our platform features a prompt and responsive support team, and clients are consistently satisfied.`,
                )}
              </p>
              <p
                className="col-1 leading-[1.6] ml-0 mt-[70px] relative row-1 w-[1120px]"
                data-node-id="342:1577"
              >
                {text(
                  `• If you check the app reviews, you'll see that the mobile version includes a compatibility tool, so you can explore questions about your future and your relationships.`,
                )}
              </p>
              <p
                className="col-1 leading-[1.6] ml-0 mt-[140px] relative row-1 w-[1120px]"
                data-node-id="342:1578"
              >
                {text(
                  "• Staryield psychics have helped millions of people, building the reputation of both our website and app. Check out Staryield reviews to see for yourself. ",
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
                  {text("Convenient 1:1 chat readings available 24/7 ")}
                </p>
              </div>
              <div
                className="content-stretch flex gap-[12px] items-center relative shrink-0 w-full"
                data-node-id="342:1608"
                data-name="Content Group / Expert guidance for your romantic relat…"
              >
                <div
                  className="bg-[var(--sage)] border border-black border-solid content-stretch flex items-center justify-center relative rounded-[999px] shrink-0 size-[24px]"
                  data-node-id="342:1609"
                  data-name="Layout / Expert Guidance For Your Romantic Relat…"
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
                  {text("Expert guidance for your romantic relationships ")}
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
                  {text("Free horoscopes and personalized astrology advice ")}
                </p>
              </div>
              <div
                className="content-stretch flex gap-[12px] items-center relative shrink-0 w-full"
                data-node-id="342:1616"
                data-name="Content Group / Free credits for your first reading"
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
                  {text("Free credits for your first reading ")}
                </p>
              </div>
              <div
                className="content-stretch flex gap-[12px] items-center relative shrink-0 w-full"
                data-node-id="342:1620"
                data-name="Content Group / An effective psychic-client matching tool"
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
                  {text("An effective psychic-client matching tool ")}
                </p>
              </div>
            </div>
          </div>
        </section>
      </div>
      <ContentLink
        className="border-b border-black border-solid border-t content-stretch flex flex-col gap-[15px] h-[411px] items-center justify-center overflow-clip px-[120px] py-[64px] relative shrink-0 w-full"
        data-node-id="342:1624"
        data-name="Button / STILL HESITATING WHETHER STARYIELD IS B…"
        label="STILL HESITATING WHETHER STARYIELD IS BETTER THAN COMPETITORS? Start First Reading Try free minutes of chat and see for yourself"
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
            {text("STILL HESITATING WHETHER STARYIELD ")}
          </p>
          <p className="leading-[normal]">
            {text("IS BETTER THAN COMPETITORS?")}
          </p>
        </div>
        <div
          className="content-stretch flex flex-col h-[77px] items-center justify-between relative shrink-0 w-[852px]"
          data-node-id="377:1500"
          data-name="Layout / STILL HESITATING WHETHER STARYIELD IS B…"
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
          <p
            className="[word-break:break-word] font-serif font-normal leading-[0] relative shrink-0 text-[#2f241c] text-[22px] text-center w-[540.545px]"
            data-node-id="342:1626"
          >
            <span className="leading-[normal]">{text(`Try `)}</span>
            <span className="font-serif font-bold leading-[normal] text-[#c29a3d]">
              {text("free minutes ")}
            </span>
            <span className="leading-[normal]">
              {text(` of chat and see for yourself`)}
            </span>
          </p>
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
              {text("Free Celestial Services ")}
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
                {text("Staryield offers a range of free services: ")}
              </p>
              <div
                className="content-stretch flex flex-col gap-[12px] items-start relative shrink-0"
                data-node-id="342:1639"
                data-name="Content Group / Personal daily horoscopes"
              >
                <div
                  className="content-stretch flex gap-[12px] items-center relative shrink-0"
                  data-node-id="342:1640"
                  data-name="Control / Personal daily horoscopes"
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
                    {text("Personal daily horoscopes ")}
                  </p>
                </div>
                <div
                  className="content-stretch flex gap-[12px] items-center relative shrink-0"
                  data-node-id="342:1644"
                  data-name="Control / The focus of the day"
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
                    {text("The focus of the day ")}
                  </p>
                </div>
                <div
                  className="content-stretch flex gap-[12px] items-center relative shrink-0"
                  data-node-id="342:1648"
                  data-name="Control / Compatibility tool"
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
                    {text("Compatibility tool ")}
                  </p>
                </div>
                <div
                  className="content-stretch flex gap-[12px] items-center relative shrink-0"
                  data-node-id="342:1652"
                  data-name="Control / Tarot of the day (Staryield App)"
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
                    {text("Tarot of the day (Staryield App) ")}
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
                  "Our platform also provides users with highly informative articles on topics ranging from the history of astrology to instructions for deep spiritual meditations. ",
                )}
              </p>
              <p className="relative shrink-0 w-full" data-node-id="342:1658">
                {text(
                  "Both our app and website are fully optimized for convenience. Discover unmatched psychic talent, absolute transparency, and genuine guidance crafted precisely for your astrological path. ",
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
                {text("Gifted Staryield Psychics ")}
              </p>
              <p
                className="font-serif font-light leading-[1.5] relative shrink-0 text-[16px]"
                data-node-id="342:1663"
              >
                {text(
                  "Over 1000 vetted advisors. Every single client review tells a true story of growth and heartfelt celestial guidance. ",
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
                {text("Smooth User Interface ")}
              </p>
              <p
                className="font-serif font-light leading-[1.5] relative shrink-0 text-[16px]"
                data-node-id="342:1666"
              >
                {text(
                  "Exceptional operational functions tailored for fluid interactions. Enjoy a completely secure, risk-free experience with zero glitches. ",
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
                {text("Wide Range of Reading Themes ")}
              </p>
              <p
                className="font-serif font-light leading-[1.5] relative shrink-0 text-[16px]"
                data-node-id="342:1669"
              >
                {text(
                  "Deep focus on love, career, finances, and life choices. Includes comprehensive tarot, aura scanning, astrology, and numerology. ",
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
                {text(`Trial Features & Offers`)}
              </p>
              <p
                className="font-serif font-light leading-[1.5] relative shrink-0 text-[16px]"
                data-node-id="342:1672"
              >
                {text(
                  "Get introduced to Staryield with a generous welcome package. Instantly receive $14 in free credits after taking our sign-up matching quiz. ",
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
                {text("Instant Live Chat Feature ")}
              </p>
              <p
                className="font-serif font-light leading-[1.5] relative shrink-0 text-[16px]"
                data-node-id="342:1675"
              >
                {text(
                  "Connect instantly on the go with real-time text readings. For slower pacing, detailed and comprehensive email readings are also available. ",
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
                {text("Get Matched Feature ")}
              </p>
              <p
                className="font-serif font-light leading-[1.5] relative shrink-0 text-[16px]"
                data-node-id="342:1678"
              >
                {text(
                  "Answer a few light questions about your current life situation and let us match you with the top readers suited to your personality. ",
                )}
              </p>
            </div>
          </div>
        </section>
      </div>
      <section
        className="bg-[var(--warm-gray)] border-b border-black border-solid content-stretch flex flex-col gap-[40px] h-[426px] items-center justify-center overflow-clip px-[160px] py-[80px] relative shrink-0 w-full figma-section"
        data-node-id="342:1679"
        data-name="Section / Legit Or Scam"
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
            {text("Staryield - Legit or Scam? ")}
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
          data-name="Layout / Legit Or Scam"
        >
          <p
            className="font-serif font-normal relative shrink-0 text-[32px] w-full"
            data-node-id="342:1688"
          >
            {text(
              "Is Staryield a secure platform? Yes. Our meticulous hiring processes ensure that only authentic, certified psychics are admitted to guide our users. ",
            )}
          </p>
          <p
            className="font-serif font-light relative shrink-0 text-[18px] w-full"
            data-node-id="342:1689"
          >
            {text(
              "Unlike competitor applications, we prioritize continuous user feedback to maintain absolute spiritual integrity. Favorable reviews across global forums corroborate our legitimacy in daily horoscope and customized psychic readings. ",
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
            {text("Spiritual Modalities ")}
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
                `Explore customized cosmic paths tailored specifically to you. Staryield's top practitioners offer specialized guidance through:`,
              )}
            </p>
            <div
              className="content-stretch flex flex-col font-serif font-light gap-[16px] items-start leading-[normal] relative shrink-0"
              data-node-id="342:1726"
              data-name="Layout / Types Of Readings"
            >
              <p className="relative shrink-0" data-node-id="342:1727">
                {text(
                  ` . Gain concrete perspective on immediate romantic and life transitions.`,
                )}
              </p>
              <p className="relative shrink-0" data-node-id="342:1728">
                {text(
                  ` . Genuine, deep emotional and spiritual connection with certified practitioners.`,
                )}
              </p>
              <p className="relative shrink-0" data-node-id="342:1729">
                {text(
                  ` . Harmonizing natural remedies with your overall astrological transits.`,
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

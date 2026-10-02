/* Figma 796:2285 — reusable responsive content regions. */
/* eslint-disable @next/next/no-img-element */
const assetPathPrefix = "https://assets.staryield.net/assets/figma";
const imgMediaDisclaimer = `${assetPathPrefix}/fb9ee.jpg`;
const imgInstanceStar = `${assetPathPrefix}/989c1.png`;
const imgDividerHorizontal = `${assetPathPrefix}/27235.svg`;
const imgDividerHorizontal1 = `${assetPathPrefix}/ea213.svg`;
export default function ScreenDesktopLegalDisclaimer() {
  return (
    <div
      className="figma-content flex w-full flex-col items-center bg-cream"
      data-node-id="796:2285"
      data-name="Screen / Desktop / Legal / Disclaimer"
    >
      <section
        className="content-stretch flex flex-col gap-[16px] items-center pb-[40px] pt-[80px] px-6 md:px-10 lg:px-20 relative shrink-0 w-full figma-section"
        data-node-id="796:2316"
        data-name="Section / Hero"
      >
        <div
          className="content-stretch flex items-center relative shrink-0"
          data-node-id="796:2317"
          data-name="Badge / STARYIELD PLATFORM POLICY"
        >
          <p
            className="[word-break:break-word] font-serif font-semibold leading-[normal] relative shrink-0 text-[#c29a3d] text-[14px] tracking-[1.4px] uppercase whitespace-nowrap"
            data-node-id="796:2320"
          >
            STARYIELD PLATFORM POLICY
          </p>
        </div>
        <h1
          className="[word-break:break-word] font-display leading-[1.2] not-italic relative shrink-0 text-[#1a1612] text-[48px] text-center whitespace-nowrap"
          data-node-id="796:2321"
        >
          Full Disclaimer
        </h1>
        <div
          className="h-0 relative shrink-0 w-[120px]"
          data-node-id="796:2322"
          data-name="Divider / Horizontal"
        >
          <div className="absolute inset-[-2px_0_0_0]">
            <img
              alt=""
              className="block max-w-none size-full"
              src={imgDividerHorizontal}
            />
          </div>
        </div>
      </section>
      <div
        className="bg-[#e0d5c3] content-stretch flex flex-col items-center pb-[100px] pt-[20px] px-[120px] relative shrink-0 w-full"
        data-node-id="796:2323"
        data-name="Container / Legal Content Container"
      >
        <div
          className="border border-[#aa9063] border-solid content-stretch drop-shadow-[0px_12px_16px_rgba(0,0,0,0.04)] flex flex-col gap-[28px] h-[1440px] items-start pb-[48px] pt-[81px] px-[48px] relative rounded-[12px] shrink-0 w-[1000px]"
          data-node-id="796:2324"
          data-name="Media / Legal Content Container"
        >
          <div
            aria-hidden
            className="absolute inset-0 pointer-events-none rounded-[12px]"
          >
            <div className="absolute bg-[var(--cream)] inset-0 rounded-[12px]" />
            <img
              alt=""
              className="absolute max-w-none mix-blend-overlay object-cover rounded-[12px] size-full"
              src={imgMediaDisclaimer}
            />
          </div>
          <p
            className="[word-break:break-word] font-serif font-semibold leading-[1.6] relative shrink-0 text-[#1a1612] text-[18px] w-full"
            data-node-id="796:2325"
          >{`THIS FULL DISCLAIMER ("DISCLAIMER") IS AN INTEGRAL PART OF, AND IS INCORPORATED BY REFERENCE INTO, THE COMPANY'S TERMS OF USE ("TERMS"). BY ACCESSING OR USING THE SERVICES, OR CLICKING “I AGREE”, CHECKING A BOX, OR USING ANY OTHER AFFIRMATIVE ACCEPTANCE MECHANISM PRESENTED THROUGH THE SERVICE, OR OTHERWISE MANIFESTING ASSENT YOU AGREE TO BE BOUND BY BOTH THIS DISCLAIMER AND THE TERMS. IN THE EVENT OF ANY CONFLICT OR INCONSISTENCY BETWEEN THIS DISCLAIMER AND THE TERMS, THE PROVISIONS OF THIS DISCLAIMER SHALL PREVAIL TO THE EXTENT OF SUCH CONFLICT. CAPITALIZED TERMS USED BUT NOT DEFINED IN THIS DISCLAIMER SHALL HAVE THE MEANINGS ASCRIBED TO THEM IN THE TERMS.`}</p>
          <div
            className="h-0 relative shrink-0 w-full"
            data-node-id="796:2326"
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
          <p
            className="[word-break:break-word] font-serif font-light leading-[1.6] relative shrink-0 text-[#2d241e] text-[17px] w-full"
            data-node-id="796:2327"
          >{`You must be at least eighteen (18) years old or otherwise capable of entering into a binding contract under applicable law to use the Company's Services.`}</p>
          <p
            className="[word-break:break-word] font-serif font-light leading-[0] relative shrink-0 text-[#2d241e] text-[17px] w-full"
            data-node-id="796:2328"
          >
            <span className="leading-[1.6]">{`The Company's Services are provided for`}</span>
            <span className="font-serif font-medium leading-[1.6]">{` entertainment purposes only`}</span>
            <span className="leading-[1.6]">{`. Any information provided within the Services or in any report or reading is not scientific and `}</span>
            <span className="font-serif font-medium leading-[1.6]">
              should not be treated as fact
            </span>
            <span className="leading-[1.6]">
              . STARYIELD’s Company Services do not constitute, and should not
              be used as a substitute for, or construed and relied upon as,
              professional advice of any kind, including, without limitation,
              psychological, medical, legal, financial, or any other form of
              regulated professional advice. If you are experiencing a personal
              crisis or require professional assistance, we strongly encourage
              you to seek help from a qualified professional in the relevant
              field.
            </span>
          </p>
          <p
            className="[word-break:break-word] font-serif font-light leading-[1.6] relative shrink-0 text-[#2d241e] text-[17px] w-full"
            data-node-id="796:2329"
          >
            You are solely responsible for selecting an advisor (psychic) and
            determining whether that advisor has the right qualifications and
            experience for your needs. We do not represent or warrant that any
            advisor (psychic) you select is licensed or qualified to provide the
            information or advice you seek, and you are encouraged to exercise
            your own judgment when selecting and engaging with an advisor
            (psychic). Certain expertise categories and specialty designations
            displayed on this platform are based solely on self-reported
            information provided directly by the respective advisors (psychics).
            The Company does not independently verify, endorse, or guarantee the
            accuracy or validity of such self-identified information.
          </p>
          <p
            className="[word-break:break-word] font-serif font-light leading-[1.6] relative shrink-0 text-[#2d241e] text-[17px] w-full"
            data-node-id="796:2330"
          >
            Advisors (psychics) are not employees of the Company. Some advisors
            (psychics) may use professional stage names and/or illustrative
            images instead of a legal name or personal photograph. The Company
            is not responsible for the truth, accuracy, completeness, safety,
            timeliness, quality, appropriateness, legality, or applicability of
            anything said, depicted, or written by users or advisors (psychics),
            including, without limitation, any information contained in
            advisors’ readings. Although the Company may take inherently
            necessary actions to ensure quality of the services, investigate
            user complaints, or prevent fraud, abuse, or harmful behavior on the
            platform, you and the advisor (psychic) are solely responsible for
            your conversations and communications.
          </p>
          <p
            className="[word-break:break-word] font-serif font-light leading-[1.6] relative shrink-0 text-[#2d241e] text-[17px] w-full"
            data-node-id="796:2331"
          >
            The Company makes no representations, warranties, or guarantees as
            to the accuracy, reliability, or outcome of the Services. Any
            recommendations or insights provided through the Services are
            intended to empower users to make their own informed decisions and
            the Company is not responsible for the personal choices or actions a
            user may take following them or a consultation with the advisor
            (psychic). The Company expressly disclaims any liability for any
            actions taken or not taken based on the Services.
          </p>
          <p
            className="[word-break:break-word] font-serif font-light leading-[1.6] relative shrink-0 text-[#2d241e] text-[17px] w-full"
            data-node-id="796:2332"
          >{`The Company's responsibility and liability relating to your use of the Services and your interactions with advisors (psychics) is limited by the terms and conditions of the Terms.`}</p>
          <div
            className="content-stretch flex items-start justify-center pt-[12px] relative shrink-0 w-full"
            data-node-id="796:2333"
            data-name="Container / Decorative Divider"
          >
            <div
              className="relative shrink-0 size-[24px]"
              data-node-id="796:2334"
              data-name="Instance / Star"
            >
              <div className="absolute inset-[-4.17%]">
                <img
                  alt=""
                  className="block max-w-none size-full"
                  height="26"
                  src={imgInstanceStar}
                  width="26"
                />
              </div>
            </div>
          </div>
        </div>
      </div>
    </div>
  );
}

/* Figma 813:2308 — reusable responsive content regions. */
/* eslint-disable @next/next/no-img-element */
import { ContentLink } from "@/components/content/content-link";
const assetPathPrefix = "https://assets.staryield.net/assets/figma";
const imgMediaTermsOfUse = `${assetPathPrefix}/fb9ee.jpg`;
const imgInstanceStar = `${assetPathPrefix}/989c1.png`;
const imgDividerHorizontal = `${assetPathPrefix}/27235.svg`;
const imgDividerHorizontal1 = `${assetPathPrefix}/ebaa9.svg`;
export default function ScreenDesktopLegalTermsOfUse() {
  return (
    <div
      className="figma-content flex w-full flex-col items-center bg-cream"
      data-node-id="813:2308"
      data-name="Screen / Desktop / Legal / Terms Of Use"
    >
      <section
        className="content-stretch flex flex-col gap-[16px] items-center pb-[40px] pt-[80px] px-6 md:px-10 lg:px-20 relative shrink-0 w-full figma-section"
        data-node-id="813:2338"
        data-name="Section / Hero"
      >
        <div
          className="content-stretch flex items-center relative shrink-0"
          data-node-id="813:2339"
          data-name="Badge / STARYIELD SYSTEM AGREEMENT"
        >
          <p
            className="[word-break:break-word] font-serif font-semibold leading-[normal] relative shrink-0 text-[#c29a3d] text-[14px] tracking-[3px] uppercase whitespace-nowrap"
            data-node-id="813:2340"
          >
            STARYIELD SYSTEM AGREEMENT
          </p>
        </div>
        <h1
          className="[word-break:break-word] font-display leading-[1.2] not-italic relative shrink-0 text-[#1a1612] text-[48px] text-center whitespace-nowrap"
          data-node-id="813:2341"
        >
          Terms of Use
        </h1>
        <div
          className="h-0 relative shrink-0 w-[120px]"
          data-node-id="813:2342"
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
        data-node-id="813:2343"
        data-name="Container / Legal Content Container"
      >
        <div
          className="border border-[#aa9063] border-solid content-stretch drop-shadow-[0px_12px_16px_rgba(0,0,0,0.04)] flex flex-col gap-[32px] items-start px-[64px] py-[80px] relative rounded-[12px] shrink-0 w-[1000px]"
          data-node-id="813:2344"
          data-name="Card / Legal Sheet"
        >
          <div
            aria-hidden
            className="absolute inset-0 pointer-events-none rounded-[12px]"
          >
            <div className="absolute bg-[var(--cream)] inset-0 rounded-[12px]" />
            <div
              className="absolute bg-size-[999.9999618530273px_834.8958014845848px] bg-top-left inset-0 mix-blend-overlay rounded-[12px]"
              style={{ backgroundImage: `url("${imgMediaTermsOfUse}")` }}
            />
          </div>
          <div
            className="[word-break:break-word] border border-[#aa9063] border-solid content-stretch flex flex-col gap-[16px] items-start p-[20px] relative rounded-[6px] shrink-0 w-full"
            data-node-id="813:2345"
            data-name="Container / Important Notices"
          >
            <p
              className="font-serif font-bold leading-[1.5] relative shrink-0 text-[#1a1612] text-[16px] w-full"
              data-node-id="813:2346"
            >
              IMPORTANT NOTICE REGARDING AUTOMATIC RENEWALS
            </p>
            <div
              className="font-serif font-normal leading-[0] relative shrink-0 text-[#2d241e] text-[14px] w-full whitespace-pre-wrap"
              data-node-id="813:2347"
            >
              <p className="leading-[1.6] mb-0">
                This Service includes subscriptions that automatically renew.
                Please read these terms and conditions of use (the “Terms”)
                carefully (in particular, Section 6) before starting a trial or
                introductory offer or completing a purchase for our platform’s
                auto-renewing subscription service. To avoid being charged, you
                must cancel your subscription before the end of your trial or
                current billing cycle. By purchasing an automatically renewing
                subscription, you acknowledge and agree to its recurring nature,
                as explained near the point of purchase. If you do not cancel in
                time, your subscription will automatically renew, and the
                applicable charges will be applied.
              </p>
              <p className="leading-[1.6] mb-0">​</p>
              <p className="leading-[1.6] mb-0">
                • If you subscribed or started a free trial via the App Store,
                refund requests are handled directly by Apple. You can submit a
                request following the instructions on the Apple Support page.
              </p>
              <p className="leading-[1.6] mb-0">
                • If you subscribed or started a free trial through the Google
                Play Store or directly via our website, please contact the
                Support center.
              </p>
              <p className="leading-[1.6] mb-0">​</p>
              <p className="leading-[1.6] mb-0">
                Deleting the app does not cancel your subscription, trial or
                introductory offer. If you intend to cancel, ensure you follow
                the appropriate cancellation process for your platform. You may
                also wish to take a screenshot of this notice for future
                reference. More details can be found in our Subscription Terms,
                which is a part of Section 5.
              </p>
              <p className="leading-[1.6] mb-0">​</p>
              <p className="leading-[1.6]">
                Our privacy practices are described in detail in our Privacy
                Policy. Please review it to understand how we collect, use, and
                share your personal information.
              </p>
            </div>
            <div
              className="font-serif font-normal leading-[0] relative shrink-0 text-[#2d241e] text-[14px] w-full whitespace-pre-wrap"
              data-node-id="813:2348"
            >
              <p className="leading-[1.6] mb-0">{`BINDING ARBITRATION & DISPUTE RESOLUTION`}</p>
              <p className="leading-[1.6] mb-0">​</p>
              <p className="leading-[1.6] mb-0">
                Section 12 of these Terms governs how disputes between you and
                the Company are resolved. In particular, it includes a binding
                arbitration agreement, which means:
              </p>
              <p className="leading-[1.6] mb-0">​</p>
              <p className="leading-[1.6] mb-0">
                • You agree to resolve disputes with us through final and
                binding arbitration, rather than in court, except for certain
                limited exceptions.
              </p>
              <p className="leading-[1.6] mb-0">
                • You waive your right to file a lawsuit or participate in a
                class action lawsuit against us.
              </p>
              <p className="leading-[1.6] mb-0">
                • You may opt out of the arbitration agreement by following the
                process outlined in Section 12.
              </p>
              <p className="leading-[1.6] mb-0">​</p>
              <p className="leading-[1.6]">
                Please read this section carefully, as it significantly affects
                your legal rights.
              </p>
            </div>
          </div>
          <div
            className="h-0 relative shrink-0 w-full"
            data-node-id="813:2349"
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
            className="[word-break:break-word] bg-[#f8f4ea] border border-[rgba(176,145,79,0.45)] border-solid content-stretch flex flex-col gap-[12px] h-[2823px] items-start overflow-clip px-[32px] py-[28px] relative rounded-[8px] shrink-0 text-[#2d241e] w-[838px]"
            data-node-id="813:2350"
            data-name="Container / Table Of Contents Navigation Links"
          >
            <p
              className="font-display leading-[1.5] min-w-full not-italic relative shrink-0 text-[18px] w-[min-content]"
              data-node-id="813:2351"
            >
              TABLE OF CONTENTS
            </p>
            <ContentLink
              className="block cursor-pointer font-serif font-semibold leading-[0] min-w-full relative shrink-0 text-[15px] text-left w-[min-content]"
              data-node-id="813:2353"
              label="1. ACCEPTANCE OF TERMS"
            >
              <h2 className="leading-[1.5]" id="acceptance-of-terms">
                1. ACCEPTANCE OF TERMS
              </h2>
            </ContentLink>
            <p
              className="flex-[1_0_0] font-serif font-normal leading-[1.5] min-h-px relative text-[14px] w-[822px] whitespace-pre-wrap"
              data-node-id="844:842"
            >{`    1.1 Additional Terms and Policies`}</p>
            <p
              className="flex-[1_0_0] font-serif font-normal leading-[1.5] min-h-px relative text-[14px] w-[822px] whitespace-pre-wrap"
              data-node-id="844:844"
            >{`    1.2 Changes to these Terms`}</p>
            <p
              className="flex-[1_0_0] font-serif font-normal leading-[1.5] min-h-px relative text-[14px] w-[822px] whitespace-pre-wrap"
              data-node-id="844:846"
            >{`    1.3 Changes to the Service`}</p>
            <ContentLink
              className="block cursor-pointer font-serif font-semibold leading-[0] min-w-full relative shrink-0 text-[15px] text-left w-[min-content]"
              data-node-id="813:2354"
              label="2. IMPORTANT DISCLAIMERS"
            >
              <h2 className="leading-[1.5]" id="important-disclaimers">
                2. IMPORTANT DISCLAIMERS
              </h2>
            </ContentLink>
            <p
              className="flex-[1_0_0] font-serif font-normal leading-[1.5] min-h-px relative text-[14px] w-[822px] whitespace-pre-wrap"
              data-node-id="844:849"
            >{`    2.1 Entertainment Purposes. No Legal, Health, Financial or Other Professional Advice`}</p>
            <p
              className="flex-[1_0_0] font-serif font-normal leading-[1.5] min-h-px relative text-[14px] w-[822px] whitespace-pre-wrap"
              data-node-id="844:851"
            >{`    2.2 Advisors (Psychics)`}</p>
            <p
              className="flex-[1_0_0] font-serif font-normal leading-[1.5] min-h-px relative text-[14px] w-[822px] whitespace-pre-wrap"
              data-node-id="844:853"
            >{`    2.3 No Guarantee of Outcomes or Service Performance`}</p>
            <ContentLink
              className="block cursor-pointer font-serif font-semibold leading-[0] min-w-full relative shrink-0 text-[15px] text-left w-[min-content]"
              data-node-id="813:2355"
              label="3. ACCOUNT REGISTRATION"
            >
              <h2 className="leading-[1.5]" id="account-registration">
                3. ACCOUNT REGISTRATION
              </h2>
            </ContentLink>
            <p
              className="flex-[1_0_0] font-serif font-normal leading-[1.5] min-h-px relative text-[14px] w-[822px] whitespace-pre-wrap"
              data-node-id="844:856"
            >{`    3.1 Creating an Account`}</p>
            <p
              className="flex-[1_0_0] font-serif font-normal leading-[1.5] min-h-px relative text-[14px] w-[822px] whitespace-pre-wrap"
              data-node-id="844:858"
            >{`    3.2 Your Responsibilities`}</p>
            <p
              className="flex-[1_0_0] font-serif font-normal leading-[1.5] min-h-px relative text-[14px] w-[822px] whitespace-pre-wrap"
              data-node-id="844:860"
            >{`    3.3 Age Restriction`}</p>
            <p
              className="flex-[1_0_0] font-serif font-normal leading-[1.5] min-h-px relative text-[14px] w-[822px] whitespace-pre-wrap"
              data-node-id="844:862"
            >{`    3.4 Account Suspension or Termination`}</p>
            <p
              className="flex-[1_0_0] font-serif font-normal leading-[1.5] min-h-px relative text-[14px] w-[822px] whitespace-pre-wrap"
              data-node-id="844:864"
            >{`    3.5 Account Security`}</p>
            <ContentLink
              className="block cursor-pointer font-serif font-semibold leading-[0] min-w-full relative shrink-0 text-[15px] text-left w-[min-content]"
              data-node-id="813:2356"
              label="4. USE OF THE SERVICE"
            >
              <h2 className="leading-[1.5]" id="use-of-the-service">
                4. USE OF THE SERVICE
              </h2>
            </ContentLink>
            <p
              className="flex-[1_0_0] font-serif font-normal leading-[1.5] min-h-px relative text-[14px] w-[822px] whitespace-pre-wrap"
              data-node-id="844:867"
            >{`    4.1 Ownership and Intellectual Property`}</p>
            <p
              className="flex-[1_0_0] font-serif font-normal leading-[1.5] min-h-px relative text-[14px] w-[822px] whitespace-pre-wrap"
              data-node-id="844:869"
            >{`    4.2 License to Use the Service`}</p>
            <p
              className="flex-[1_0_0] font-serif font-normal leading-[1.5] min-h-px relative text-[14px] w-[822px] whitespace-pre-wrap"
              data-node-id="844:871"
            >{`    4.3 User-Generated Content`}</p>
            <p
              className="flex-[1_0_0] font-serif font-normal leading-[1.5] min-h-px relative text-[14px] w-[822px] whitespace-pre-wrap"
              data-node-id="844:873"
            >{`    4.4 User Reviews and Testimonials`}</p>
            <p
              className="flex-[1_0_0] font-serif font-normal leading-[1.5] min-h-px relative text-[14px] w-[822px] whitespace-pre-wrap"
              data-node-id="844:875"
            >{`    4.5 User Responsibilities and Rights`}</p>
            <p
              className="flex-[1_0_0] font-serif font-normal leading-[1.5] min-h-px relative text-[14px] w-[822px] whitespace-pre-wrap"
              data-node-id="844:877"
            >{`    4.6 Content Moderation and Restrictions`}</p>
            <p
              className="flex-[1_0_0] font-serif font-normal leading-[1.5] min-h-px relative text-[14px] w-[822px] whitespace-pre-wrap"
              data-node-id="844:879"
            >{`    4.7 Prohibited Conduct`}</p>
            <p
              className="flex-[1_0_0] font-serif font-normal leading-[1.5] min-h-px relative text-[14px] w-[822px] whitespace-pre-wrap"
              data-node-id="844:881"
            >{`    4.8 Service Availability and Modifications`}</p>
            <p
              className="flex-[1_0_0] font-serif font-normal leading-[1.5] min-h-px relative text-[14px] w-[822px] whitespace-pre-wrap"
              data-node-id="844:883"
            >{`    4.9 Risks and Disclaimer of Liability`}</p>
            <p
              className="flex-[1_0_0] font-serif font-normal leading-[1.5] min-h-px relative text-[14px] w-[822px] whitespace-pre-wrap"
              data-node-id="844:885"
            >{`    4.10 Customer Support`}</p>
            <ContentLink
              className="block cursor-pointer font-serif font-semibold leading-[0] min-w-full relative shrink-0 text-[15px] text-left w-[min-content]"
              data-node-id="813:2357"
              label="5. THIRD-PARTY SERVICES, MATERIALS, AND ADVERTISING"
            >
              <h2
                className="leading-[1.5]"
                id="third-party-services-materials-and-advertising"
              >
                5. THIRD-PARTY SERVICES, MATERIALS, AND ADVERTISING
              </h2>
            </ContentLink>
            <p
              className="flex-[1_0_0] font-serif font-normal leading-[1.5] min-h-px relative text-[14px] w-[822px] whitespace-pre-wrap"
              data-node-id="844:888"
            >{`    5.1 No Endorsement or Responsibility`}</p>
            <p
              className="flex-[1_0_0] font-serif font-normal leading-[1.5] min-h-px relative text-[14px] w-[822px] whitespace-pre-wrap"
              data-node-id="844:890"
            >{`    5.2 Third-Party Links and Advertising`}</p>
            <p
              className="flex-[1_0_0] font-serif font-normal leading-[1.5] min-h-px relative text-[14px] w-[822px] whitespace-pre-wrap"
              data-node-id="844:892"
            >{`    5.3 No Liability for Third-Party Content`}</p>
            <p
              className="flex-[1_0_0] font-serif font-normal leading-[1.5] min-h-px relative text-[14px] w-[822px] whitespace-pre-wrap"
              data-node-id="844:894"
            >{`    5.4 Use of Third-Party Services at Your Own Risk`}</p>
            <ContentLink
              className="block cursor-pointer font-serif font-semibold leading-[0] min-w-full relative shrink-0 text-[15px] text-left w-[min-content]"
              data-node-id="813:2358"
              label="6. SUBSCRIPTION FEES, PAYMENTS, AND REFUNDS"
            >
              <h2
                className="leading-[1.5]"
                id="subscription-fees-payments-and-refunds"
              >
                6. SUBSCRIPTION FEES, PAYMENTS, AND REFUNDS
              </h2>
            </ContentLink>
            <p
              className="flex-[1_0_0] font-serif font-normal leading-[1.5] min-h-px relative text-[14px] w-[822px] whitespace-pre-wrap"
              data-node-id="844:897"
            >{`    6.1 Balance Terms`}</p>
            <p
              className="flex-[1_0_0] font-serif font-light leading-[1.5] min-h-px relative text-[14px] w-[806px] whitespace-pre-wrap"
              data-node-id="844:899"
            >{`       6.1.1 Account Balance`}</p>
            <p
              className="flex-[1_0_0] font-serif font-light leading-[1.5] min-h-px relative text-[14px] w-[806px] whitespace-pre-wrap"
              data-node-id="844:901"
            >{`       6.1.2 Auto-Refill`}</p>
            <p
              className="flex-[1_0_0] font-serif font-light leading-[1.5] min-h-px relative text-[14px] w-[806px] whitespace-pre-wrap"
              data-node-id="844:903"
            >{`       6.1.3 Payment Method`}</p>
            <p
              className="flex-[1_0_0] font-serif font-light leading-[1.5] min-h-px relative text-[14px] w-[806px] whitespace-pre-wrap"
              data-node-id="844:905"
            >{`       6.1.4 How to Cancel`}</p>
            <p
              className="flex-[1_0_0] font-serif font-light leading-[1.5] min-h-px relative text-[14px] w-[806px] whitespace-pre-wrap"
              data-node-id="844:907"
            >{`       6.1.5 Limitations`}</p>
            <p
              className="flex-[1_0_0] font-serif font-normal leading-[1.5] min-h-px relative text-[14px] w-[822px] whitespace-pre-wrap"
              data-node-id="844:909"
            >{`    6.2 Subscription Terms`}</p>
            <p
              className="flex-[1_0_0] font-serif font-light leading-[1.5] min-h-px relative text-[14px] w-[806px] whitespace-pre-wrap"
              data-node-id="844:911"
            >{`       6.2.1 Subscription Options, Purchases and Refunds`}</p>
            <p
              className="flex-[1_0_0] font-serif font-light leading-[1.5] min-h-px relative text-[14px] w-[806px] whitespace-pre-wrap"
              data-node-id="844:913"
            >{`       6.2.2 Purchases and Payment Processing`}</p>
            <p
              className="flex-[1_0_0] font-serif font-light leading-[1.5] min-h-px relative text-[14px] w-[806px] whitespace-pre-wrap"
              data-node-id="844:915"
            >{`       6.2.3 Auto-Renewal, Subscription Continuity and Cancellation`}</p>
            <p
              className="flex-[1_0_0] font-serif font-light leading-[1.5] min-h-px relative text-[14px] w-[806px] whitespace-pre-wrap"
              data-node-id="844:917"
            >{`       6.2.4 Add-On Items and Additional Services`}</p>
            <p
              className="flex-[1_0_0] font-serif font-light leading-[1.5] min-h-px relative text-[14px] w-[806px] whitespace-pre-wrap"
              data-node-id="844:919"
            >{`       6.2.5 Free Trials and Promotional Offers`}</p>
            <p
              className="flex-[1_0_0] font-serif font-light leading-[1.5] min-h-px relative text-[14px] w-[806px] whitespace-pre-wrap"
              data-node-id="844:921"
            >{`       6.2.6 Promotional Codes`}</p>
            <p
              className="flex-[1_0_0] font-serif font-light leading-[1.5] min-h-px relative text-[14px] w-[806px] whitespace-pre-wrap"
              data-node-id="844:923"
            >{`       6.2.7 Changes to Subscription Fees`}</p>
            <p
              className="flex-[1_0_0] font-serif font-light leading-[1.5] min-h-px relative text-[14px] w-[806px] whitespace-pre-wrap"
              data-node-id="844:925"
            >{`       6.2.8 Failure to Pay and Service Termination`}</p>
            <p
              className="flex-[1_0_0] font-serif font-normal leading-[1.5] min-h-px relative text-[14px] w-[822px] whitespace-pre-wrap"
              data-node-id="844:927"
            >{`    6.3 General Terms`}</p>
            <p
              className="flex-[1_0_0] font-serif font-light leading-[1.5] min-h-px relative text-[14px] w-[806px] whitespace-pre-wrap"
              data-node-id="844:929"
            >{`       6.3.1 Refunds`}</p>
            <p
              className="flex-[1_0_0] font-serif font-light leading-[1.5] min-h-px relative text-[14px] w-[806px] whitespace-pre-wrap"
              data-node-id="844:931"
            >{`       6.3.2 Chargebacks and Payment Disputes`}</p>
            <p
              className="flex-[1_0_0] font-serif font-light leading-[1.5] min-h-px relative text-[14px] w-[806px] whitespace-pre-wrap"
              data-node-id="844:933"
            >{`       6.3.3 Taxes`}</p>
            <ContentLink
              className="block cursor-pointer font-serif font-semibold leading-[0] min-w-full relative shrink-0 text-[15px] text-left w-[min-content]"
              data-node-id="813:2359"
              label="7. USER REPRESENTATIONS AND RESTRICTIONS"
            >
              <h2
                className="leading-[1.5]"
                id="user-representations-and-restrictions"
              >
                7. USER REPRESENTATIONS AND RESTRICTIONS
              </h2>
            </ContentLink>
            <p
              className="flex-[1_0_0] font-serif font-normal leading-[1.5] min-h-px relative text-[14px] w-[822px] whitespace-pre-wrap"
              data-node-id="844:936"
            >{`    7.1 Permitted Use of the Service`}</p>
            <p
              className="flex-[1_0_0] font-serif font-normal leading-[1.5] min-h-px relative text-[14px] w-[822px] whitespace-pre-wrap"
              data-node-id="844:938"
            >{`    7.2 Prohibited Conduct`}</p>
            <p
              className="flex-[1_0_0] font-serif font-normal leading-[1.5] min-h-px relative text-[14px] w-[822px] whitespace-pre-wrap"
              data-node-id="844:940"
            >{`    7.3 Respectful Conduct Towards Customer Support`}</p>
            <ContentLink
              className="block cursor-pointer font-serif font-semibold leading-[0] min-w-full relative shrink-0 text-[15px] text-left w-[min-content]"
              data-node-id="813:2360"
              label="8. DISCLAIMER OF WARRANTIES"
            >
              <h2 className="leading-[1.5]" id="disclaimer-of-warranties">
                8. DISCLAIMER OF WARRANTIES
              </h2>
            </ContentLink>
            <p
              className="flex-[1_0_0] font-serif font-normal leading-[1.5] min-h-px relative text-[14px] w-[822px] whitespace-pre-wrap"
              data-node-id="844:943"
            >{`    8.1 General Disclaimers`}</p>
            <p
              className="flex-[1_0_0] font-serif font-normal leading-[1.5] min-h-px relative text-[14px] w-[822px] whitespace-pre-wrap"
              data-node-id="844:945"
            >{`    8.2 Service Modifications and Updates`}</p>
            <p
              className="flex-[1_0_0] font-serif font-normal leading-[1.5] min-h-px relative text-[14px] w-[822px] whitespace-pre-wrap"
              data-node-id="844:947"
            >{`    8.3 Consumer Protection and Non-Waivable Rights`}</p>
            <ContentLink
              className="block cursor-pointer font-serif font-semibold leading-[0] min-w-full relative shrink-0 text-[15px] text-left w-[min-content]"
              data-node-id="813:2361"
              label="9. LIMITATION OF LIABILITY"
            >
              <h2 className="leading-[1.5]" id="limitation-of-liability">
                9. LIMITATION OF LIABILITY
              </h2>
            </ContentLink>
            <p
              className="flex-[1_0_0] font-serif font-normal leading-[1.5] min-h-px relative text-[14px] w-[822px] whitespace-pre-wrap"
              data-node-id="844:950"
            >{`    9.1 Limitation of Aggregate Liability`}</p>
            <p
              className="flex-[1_0_0] font-serif font-normal leading-[1.5] min-h-px relative text-[14px] w-[822px] whitespace-pre-wrap"
              data-node-id="844:952"
            >{`    9.2 Waiver of Unknown Claims (California Residents)`}</p>
            <p
              className="flex-[1_0_0] font-serif font-normal leading-[1.5] min-h-px relative text-[14px] w-[822px] whitespace-pre-wrap"
              data-node-id="844:954"
            >{`    9.3 Jurisdiction-Specific Exceptions`}</p>
            <ContentLink
              className="block cursor-pointer font-serif font-semibold leading-[0] min-w-full relative shrink-0 text-[15px] text-left w-[min-content]"
              data-node-id="813:2362"
              label="10. INDEMNIFICATION"
            >
              <h2 className="leading-[1.5]" id="indemnification">
                10. INDEMNIFICATION
              </h2>
            </ContentLink>
            <ContentLink
              className="block cursor-pointer font-serif font-semibold leading-[0] min-w-full relative shrink-0 text-[15px] text-left w-[min-content]"
              data-node-id="813:2363"
              label="11. INTERNATIONAL USE"
            >
              <h2 className="leading-[1.5]" id="international-use">
                11. INTERNATIONAL USE
              </h2>
            </ContentLink>
            <ContentLink
              className="block cursor-pointer font-serif font-semibold leading-[0] min-w-full relative shrink-0 text-[15px] text-left w-[min-content]"
              data-node-id="813:2364"
              label="12. INFORMAL DISPUTE RESOLUTION PROCEDURES AND ARBITRATION"
            >
              <h2
                className="leading-[1.5]"
                id="informal-dispute-resolution-procedures-and-arbitration"
              >
                12. INFORMAL DISPUTE RESOLUTION PROCEDURES AND ARBITRATION
              </h2>
            </ContentLink>
            <p
              className="flex-[1_0_0] font-serif font-normal leading-[1.5] min-h-px relative text-[14px] w-[822px] whitespace-pre-wrap"
              data-node-id="844:959"
            >{`    12.1 Mandatory Pre-Filing Notice Procedure`}</p>
            <p
              className="flex-[1_0_0] font-serif font-normal leading-[1.5] min-h-px relative text-[14px] w-[822px] whitespace-pre-wrap"
              data-node-id="844:961"
            >{`    12.2 Small Claims Court`}</p>
            <p
              className="flex-[1_0_0] font-serif font-normal leading-[1.5] min-h-px relative text-[14px] w-[822px] whitespace-pre-wrap"
              data-node-id="844:963"
            >{`    12.3 What is Arbitration?`}</p>
            <p
              className="flex-[1_0_0] font-serif font-normal leading-[1.5] min-h-px relative text-[14px] w-[822px] whitespace-pre-wrap"
              data-node-id="844:965"
            >{`    12.4 CLASS ACTION AND JURY TRIAL WAIVER`}</p>
            <p
              className="flex-[1_0_0] font-serif font-normal leading-[1.5] min-h-px relative text-[14px] w-[822px] whitespace-pre-wrap"
              data-node-id="844:967"
            >{`    12.5 Arbitration Procedure`}</p>
            <p
              className="flex-[1_0_0] font-serif font-normal leading-[1.5] min-h-px relative text-[14px] w-[822px] whitespace-pre-wrap"
              data-node-id="844:969"
            >{`    12.6 Decision of the Arbitrator`}</p>
            <p
              className="flex-[1_0_0] font-serif font-normal leading-[1.5] min-h-px relative text-[14px] w-[822px] whitespace-pre-wrap"
              data-node-id="844:971"
            >{`    12.7 Fees`}</p>
            <p
              className="flex-[1_0_0] font-serif font-normal leading-[1.5] min-h-px relative text-[14px] w-[822px] whitespace-pre-wrap"
              data-node-id="844:973"
            >{`    12.8 Confidentiality`}</p>
            <p
              className="flex-[1_0_0] font-serif font-normal leading-[1.5] min-h-px relative text-[14px] w-[822px] whitespace-pre-wrap"
              data-node-id="844:975"
            >{`    12.9 Settlement Offers and Offers of Judgment`}</p>
            <p
              className="flex-[1_0_0] font-serif font-normal leading-[1.5] min-h-px relative text-[14px] w-[822px] whitespace-pre-wrap"
              data-node-id="844:977"
            >{`    12.10 Additional Procedures for Mass Arbitration Filings`}</p>
            <p
              className="flex-[1_0_0] font-serif font-normal leading-[1.5] min-h-px relative text-[14px] w-[822px] whitespace-pre-wrap"
              data-node-id="844:979"
            >{`    12.11 Opting Out of this Arbitration Agreement`}</p>
            <p
              className="flex-[1_0_0] font-serif font-normal leading-[1.5] min-h-px relative text-[14px] w-[822px] whitespace-pre-wrap"
              data-node-id="844:981"
            >{`    12.12 Governing Law`}</p>
            <ContentLink
              className="block cursor-pointer font-serif font-semibold leading-[0] min-w-full relative shrink-0 text-[15px] text-left w-[min-content]"
              data-node-id="813:2365"
              label="13. FOR EEA AND UK RESIDENTS"
            >
              <h2 className="leading-[1.5]" id="for-eea-and-uk-residents">
                13. FOR EEA AND UK RESIDENTS
              </h2>
            </ContentLink>
            <ContentLink
              className="block cursor-pointer font-serif font-semibold leading-[0] min-w-full relative shrink-0 text-[15px] text-left w-[min-content]"
              data-node-id="813:2366"
              label="14. FOR CALIFORNIA RESIDENTS"
            >
              <h2 className="leading-[1.5]" id="for-california-residents">
                14. FOR CALIFORNIA RESIDENTS
              </h2>
            </ContentLink>
            <ContentLink
              className="block cursor-pointer font-serif font-semibold leading-[0] min-w-full relative shrink-0 text-[15px] text-left w-[min-content]"
              data-node-id="813:2367"
              label="15. LIMITATION ON CLAIMS PERIOD"
            >
              <h2 className="leading-[1.5]" id="limitation-on-claims-period">
                15. LIMITATION ON CLAIMS PERIOD
              </h2>
            </ContentLink>
            <ContentLink
              className="block cursor-pointer font-serif font-semibold leading-[0] min-w-full relative shrink-0 text-[15px] text-left w-[min-content]"
              data-node-id="813:2368"
              label="16. MISCELLANEOUS"
            >
              <h2 className="leading-[1.5]" id="miscellaneous">
                16. MISCELLANEOUS
              </h2>
            </ContentLink>
            <h2
              className="flex-[1_0_0] font-serif font-semibold leading-[1.5] min-h-px relative text-[15px] w-[822px]"
              data-node-id="844:987"
              id="contact-information"
            >
              17. Contact Information
            </h2>
          </div>
          <div
            className="h-0 relative shrink-0 w-full"
            data-node-id="813:2369"
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
          <section
            className="[word-break:break-word] content-stretch flex flex-col gap-[12px] items-start relative shrink-0 w-full figma-section"
            data-node-id="813:2370"
            data-name="Section / 1"
          >
            <h2
              className="font-display leading-[normal] not-italic relative shrink-0 text-[#1a1612] text-[16px] w-full"
              data-node-id="813:2371"
              id="acceptance-of-terms"
            >
              1. ACCEPTANCE OF TERMS
            </h2>
            <div
              className="font-serif font-normal leading-[0] relative shrink-0 text-[#2d241e] text-[0px] w-full whitespace-pre-wrap"
              data-node-id="813:2372"
            >
              <p className="leading-[1.6] mb-0 text-[15px]">
                The provisions of the “Terms” govern the relationship between
                you and Spiritual Staryield Limited (address: 62 Athalassas
                Avenue, Mezzanine Floor, Strovolos, 2012 Nicosia, Cyprus) (“we”,
                “us”, “our” or the “Company”) regarding your use of the
                Company’s mobile applications, websites, devices and related
                services (the “App” or “Service”), including all information,
                text, graphics, video, music, software and other content and
                services, available for your use (the “Content”).
              </p>
              <p className="leading-[1.6] mb-0 text-[15px]">​</p>
              <p className="leading-[1.6] mb-0 text-[15px]">
                By accessing or using any part of the Service, or clicking “I
                agree”, checking a box, or using any other acceptance mechanism
                presented through the Service, or otherwise manifesting assent
                you acknowledge that you have read, understood, and agree to be
                bound by these Terms, forming a legally binding agreement
                between you and the Company. If you do not agree to these Terms,
                you must immediately stop using the Service, delete your
                account, and cancel any active subscriptions.
              </p>
              <p className="leading-[1.6] mb-0 text-[15px]">​</p>
              <p className="leading-[1.6] mb-0 text-[15px]">
                These Terms were originally drafted in English. Any translation
                from the English version is provided for your convenience only.
                If there is any conflict between the English language version of
                these Terms and a version translated into another language, the
                English-language version will prevail.
              </p>
              <p className="leading-[1.6] mb-0 text-[15px]">​</p>
              <p className="font-serif font-semibold leading-[1.6] mb-[8px] text-[16px]">
                1.1 Additional Terms and Policies
              </p>
              <p className="leading-[1.6] mb-0 text-[15px]">​</p>
              <p className="leading-[1.6] mb-0 text-[15px]">
                Our Privacy Policy forms an integral part of these Terms and
                describes how we collect, use, and protect your personal data.
                We may also post additional policies, supplemental terms, or
                notices on the Service from time to time. Such terms are hereby
                incorporated by reference and will apply to your use of the
                Service.
              </p>
              <p className="leading-[1.6] mb-0 text-[15px]">​</p>
              <p className="font-serif font-semibold leading-[1.6] mb-[8px] text-[16px]">
                1.2 Changes to these Terms
              </p>
              <p className="leading-[1.6] mb-0 text-[15px]">​</p>
              <p className="leading-[1.6] mb-0 text-[15px]">
                We may update, modify, or remove portions of these Terms at our
                sole discretion, to the extent permitted by applicable law. This
                may occur when we introduce or discontinue features,
                technologies, or services, to comply with legal, regulatory, or
                contractual requirements, or in response to exceptional or
                unforeseen circumstances. Where required by law, we will notify
                you of such changes.
              </p>
              <p className="leading-[1.6] mb-0 text-[15px]">​</p>
              <p className="leading-[1.6] mb-0 text-[15px]">
                Unless stated otherwise, we will indicate updates by revising
                the “Last Updated” date of these Terms. You acknowledge and
                agree that it is your responsibility to review the Terms
                regularly for any updates. Unless specified otherwise, the
                updated Terms take effect once posted on the Service. By
                continuing to use the Service after the updates become
                effective, you agree to the revised Terms. If you do not agree,
                you must stop using the Service immediately, delete your account
                and cancel your subscription.
              </p>
              <p className="leading-[1.6] mb-0 text-[15px]">​</p>
              <p className="font-serif font-semibold leading-[1.6] mb-[8px] text-[16px]">
                1.3 Changes to the Service
              </p>
              <p className="leading-[1.6] mb-0 text-[15px]">​</p>
              <p className="leading-[1.6] text-[15px]">
                We may also update, change, suspend or discontinue the Service
                (or any part, content or feature) at any time, without notice
                and without liability to you or anyone else (for example, to
                offer or test new or different features, technologies, or
                services, to repair, improve or further develop the Service, to
                comply with legal, regulatory or contractual requirements, or in
                response to exceptional or unforeseen circumstances). Some
                services and features may not be available in all countries, in
                all languages, or in all operating systems.
              </p>
            </div>
          </section>
          <section
            className="[word-break:break-word] content-stretch flex flex-col gap-[12px] items-start relative shrink-0 w-full figma-section"
            data-node-id="813:2373"
            data-name="Section / 2"
          >
            <h2
              className="font-display leading-[normal] not-italic relative shrink-0 text-[#1a1612] text-[16px] w-full"
              data-node-id="813:2374"
              id="important-disclaimers"
            >
              2. IMPORTANT DISCLAIMERS
            </h2>
            <div
              className="font-serif font-normal leading-[0] relative shrink-0 text-[#2d241e] text-[0px] w-full whitespace-pre-wrap"
              data-node-id="813:2375"
            >
              <p className="font-serif font-semibold leading-[1.6] mb-[8px] text-[16px]">
                2.1 Entertainment Purposes. No Legal, Health, Financial or Other
                Professional Advice
              </p>
              <p className="leading-[1.6] mb-0 text-[15px]">​</p>
              <p className="leading-[1.6] mb-0 text-[15px]">
                This Service is provided for entertainment purposes only. Any
                information provided within the Service or in any report or
                reading is not scientific and should not be treated as fact.
                This Service does not constitute, and should not be used as a
                substitute for, or construed and relied upon as, professional
                advice of any kind, including, without limitation,
                psychological, medical, legal, financial, or any other form of
                regulated professional advice. If you are experiencing a
                personal crisis or require professional assistance, we strongly
                encourage you to seek help from a qualified professional in the
                relevant field.
              </p>
              <p className="leading-[1.6] mb-0 text-[15px]">​</p>
              <p className="font-serif font-semibold leading-[1.6] mb-[8px] text-[16px]">
                2.2 Advisors (Psychics)
              </p>
              <p className="leading-[1.6] mb-0 text-[15px]">​</p>
              <p className="leading-[1.6] mb-0 text-[15px]">
                You are solely responsible for selecting an advisor (psychic)
                and determining whether that advisor has the right
                qualifications and experience for your needs. We do not
                represent or warrant that any advisor (psychic) you select is
                licensed or qualified to provide the information or advice you
                seek, and you are encouraged to exercise your own judgment when
                selecting and engaging with an advisor (psychic).
              </p>
              <p className="leading-[1.6] mb-0 text-[15px]">​</p>
              <p className="leading-[1.6] mb-0 text-[15px]">
                Advisors (psychics) are not employees of the Company. Some
                advisors (psychics) may use professional stage names and/or
                illustrative images instead of a legal name or personal
                photograph. The Company is not responsible for the truth,
                accuracy, completeness, safety, timeliness, quality,
                appropriateness, legality, or applicability of anything said,
                depicted, or written by users or advisors (psychics), including,
                without limitation, any information contained in advisors’
                readings. Although the Company may take inherently necessary
                actions to ensure quality of the services, investigate user
                complaints, or prevent fraud, abuse, or harmful behavior on the
                platform, you and the advisor (psychic) are solely responsible
                for your conversations and communications.
              </p>
              <p className="leading-[1.6] mb-0 text-[15px]">​</p>
              <p className="font-serif font-semibold leading-[1.6] mb-[8px] text-[16px]">
                2.3 No Guarantee of Outcomes or Service Performance
              </p>
              <p className="leading-[1.6] mb-0 text-[15px]">​</p>
              <p className="leading-[1.6] mb-0 text-[15px]">
                The Company makes no representations, warranties, or guarantees
                as to the accuracy, reliability, or outcome of the Services. Any
                recommendations or insights provided through the Services are
                intended to empower users to make their own informed decisions
                and the Company is not responsible for the personal choices or
                actions a user may take following them or a consultation with
                the advisor (psychic). The Company expressly disclaims any
                liability for any actions taken or not taken based on the
                Services.
              </p>
              <p className="leading-[1.6] mb-0 text-[15px]">​</p>
              <p className="leading-[1.6] text-[15px]">
                We make no guarantees that the Service will meet your
                requirements, will be uninterrupted, timely, secure, or
                error-free, and that the quality of any products, services,
                information, or other material purchased or obtained by you
                through the Service will meet your expectations or will provide
                any benefit.
              </p>
            </div>
          </section>
          <section
            className="[word-break:break-word] content-stretch flex flex-col gap-[12px] items-start relative shrink-0 w-full figma-section"
            data-node-id="813:2376"
            data-name="Section / 3"
          >
            <h2
              className="font-display leading-[normal] not-italic relative shrink-0 text-[#1a1612] text-[16px] w-full"
              data-node-id="813:2377"
              id="account-registration"
            >
              3. ACCOUNT REGISTRATION
            </h2>
            <div
              className="font-serif font-normal leading-[0] relative shrink-0 text-[#2d241e] text-[0px] w-full whitespace-pre-wrap"
              data-node-id="813:2378"
            >
              <p className="font-serif font-semibold leading-[1.6] mb-[8px] text-[16px]">
                3.1 Creating an Account
              </p>
              <p className="leading-[1.6] mb-0 text-[15px]">​</p>
              <p className="leading-[1.6] mb-0 text-[15px]">
                To access certain features of the Service, you may be required
                to register an account (“Account”) and provide accurate and
                complete information during the registration process.
              </p>
              <p className="leading-[1.6] mb-0 text-[15px]">​</p>
              <p className="font-serif font-semibold leading-[1.6] mb-[8px] text-[16px]">
                3.2 Your Responsibilities
              </p>
              <p className="leading-[1.6] mb-0 text-[15px]">​</p>
              <p className="leading-[1.6] mb-0 text-[15px]">
                By creating an Account, you represent and warrant that: (1) the
                information you provide is truthful, accurate, and up to date;
                (2) You will update your Account information as needed to keep
                it accurate; (3) Your use of the Service complies with all
                applicable laws, regulations, and these Terms. Failure to
                provide or maintain accurate information may impact the
                functionality of the Service, and we may be unable to notify you
                of important updates.
              </p>
              <p className="leading-[1.6] mb-0 text-[15px]">​</p>
              <p className="font-serif font-semibold leading-[1.6] mb-[8px] text-[16px]">
                3.3 Age Restriction
              </p>
              <p className="leading-[1.6] mb-0 text-[15px]">​</p>
              <p className="leading-[1.6] mb-0 text-[15px]">
                The Service is intended for users aged 18 and older. By creating
                an Account, you confirm that you are at least 18 years old and
                have the legal authority to enter into and comply with these
                Terms. If you are under 18, you are prohibited from using the
                Service.
              </p>
              <p className="leading-[1.6] mb-0 text-[15px]">​</p>
              <p className="font-serif font-semibold leading-[1.6] mb-[8px] text-[16px]">
                3.4 Account Suspension or Termination
              </p>
              <p className="leading-[1.6] mb-0 text-[15px]">​</p>
              <p className="leading-[1.6] mb-0 text-[15px]">
                We reserve the right to suspend or terminate your Account and
                restrict your access to the Service at our discretion, with or
                without prior notice, if we determine that you have violated
                these Terms or any applicable laws. This includes cases where
                you have provided false, misleading, or incomplete information
                during registration or engaged in fraudulent, abusive, or
                unauthorized activity on the Service. Termination may result in
                the loss of access to your data, content, or any benefits
                associated with the Service, and we are not responsible for any
                consequences resulting from such actions.
              </p>
              <p className="leading-[1.6] mb-0 text-[15px]">​</p>
              <p className="font-serif font-semibold leading-[1.6] mb-[8px] text-[16px]">
                3.5 Account Security
              </p>
              <p className="leading-[1.6] mb-0 text-[15px]">​</p>
              <p className="leading-[1.6] text-[15px]">
                You are responsible for maintaining the confidentiality of your
                Account credentials, including login details and passwords, and
                for all activity conducted under your Account. You should not
                share your login information with anyone, as you assume full
                responsibility for any actions taken through your Account. If
                you suspect unauthorized access or a security breach, you must
                notify us immediately at our Support center. We are not liable
                for any loss, unauthorized transactions, or damage resulting
                from access to your Account due to your failure to protect your
                credentials. It is your responsibility to use secure passwords
                and take necessary precautions to prevent unauthorized access.
              </p>
            </div>
          </section>
          <section
            className="[word-break:break-word] content-stretch flex flex-col gap-[12px] items-start relative shrink-0 w-full figma-section"
            data-node-id="813:2379"
            data-name="Section / 4"
          >
            <h2
              className="font-display leading-[normal] not-italic relative shrink-0 text-[#1a1612] text-[16px] w-full"
              data-node-id="813:2380"
              id="use-of-the-service"
            >
              4. USE OF THE SERVICE
            </h2>
            <div
              className="font-serif font-normal leading-[0] relative shrink-0 text-[#2d241e] text-[0px] w-full whitespace-pre-wrap"
              data-node-id="813:2381"
            >
              <p className="font-serif font-semibold leading-[1.6] mb-[8px] text-[16px]">
                4.1 Ownership and Intellectual Property
              </p>
              <p className="leading-[1.6] mb-0 text-[15px]">​</p>
              <p className="leading-[1.6] mb-0 text-[15px]">
                The Service, including its software, content, text, images,
                logos, trademarks, any associated materials, and any other
                Content remains the exclusive property of the Company or its
                licensors. Accessing or using the Service does not grant you
                ownership of any intellectual property rights beyond what is
                explicitly stated in these Terms. You may not copy, modify,
                distribute, sell, or reverse-engineer, decompile or disassemble
                any portion of the Service unless expressly permitted.
              </p>
              <p className="leading-[1.6] mb-0 text-[15px]">​</p>
              <p className="font-serif font-semibold leading-[1.6] mb-[8px] text-[16px]">
                4.2 License to Use the Service
              </p>
              <p className="leading-[1.6] mb-0 text-[15px]">​</p>
              <p className="leading-[1.6] mb-0 text-[15px]">
                You are granted a limited, non-exclusive, non-transferable,
                revocable license to access and use the Service for personal,
                non-commercial purposes. This license allows you to use Service
                solely for your personal, non-commercial purposes on your
                personal device, but does not permit sublicensing, resale,
                modification, or unauthorized use. Any breach of these Terms may
                result in the immediate suspension or termination of your access
                to the Service.
              </p>
              <p className="leading-[1.6] mb-0 text-[15px]">​</p>
              <p className="font-serif font-semibold leading-[1.6] mb-[8px] text-[16px]">
                4.3 User-Generated Content
              </p>
              <p className="leading-[1.6] mb-0 text-[15px]">​</p>
              <p className="leading-[1.6] mb-0 text-[15px]">
                The Service may allow you to submit, upload, or share text,
                images, messages, feedback, and other materials (“User
                Content”). By submitting User Content, you grant the Company,
                its sublicensees, successors, and assigns a royalty-free,
                perpetual, irrevocable, sublicensable, assignable, worldwide
                license to use, reproduce, modify, adapt, translate, publish,
                distribute, publicly display, and create derivative works from
                your content in any form, media, or technology, whether now
                known or later developed. This license includes the right to
                incorporate User Content into other works and services,
                including marketing, analytics, and operational improvements.
                However, this license explicitly excludes any personal data as
                defined under applicable privacy laws, which will be handled in
                accordance with the Company’s Privacy Policy.
              </p>
              <p className="leading-[1.6] mb-0 text-[15px]">​</p>
              <p className="font-serif font-semibold leading-[1.6] mb-[8px] text-[16px]">
                4.4 User Reviews and Testimonials
              </p>
              <p className="leading-[1.6] mb-0 text-[15px]">​</p>
              <p className="leading-[1.6] mb-0 text-[15px]">
                By submitting, posting, or otherwise providing any review,
                rating, comment, testimonial, or other feedback (“Review”) about
                the Service on any platform, including but not limited to the
                Apple App Store, Google Play Store, and other mobile application
                marketplaces, websites, social media platforms, or directly to
                the Company, you grant the Company and its affiliates a
                non-exclusive, worldwide, perpetual, irrevocable, royalty-free,
                sublicensable, and transferable right to use, reproduce, modify,
                adapt, publish, translate, distribute, publicly perform,
                publicly display, and create derivative works from such Reviews
                for any lawful purpose, including but not limited to marketing,
                advertising, promotional activities, product development, and
                customer engagement, in any media now known or later developed,
                without further notice, attribution, or compensation to you.
              </p>
              <p className="leading-[1.6] mb-0 text-[15px]">​</p>
              <p className="leading-[1.6] mb-0 text-[15px]">
                You acknowledge and agree that:
              </p>
              <p className="leading-[1.6] mb-0 text-[15px]">​</p>
              <p className="leading-[1.6] mb-0 text-[15px]">
                • The Company is not obligated to use, display, or maintain any
                Review and may remove or edit Reviews at its discretion.
              </p>
              <p className="leading-[1.6] mb-0 text-[15px]">
                • The Company may use Reviews in conjunction with your publicly
                displayed username, profile picture, or other identifying
                information (if available), unless you request anonymity in
                writing.
              </p>
              <p className="leading-[1.6] mb-0 text-[15px]">
                • The Company is not responsible for the content of Reviews
                posted by users and does not endorse any opinions expressed
                therein.
              </p>
              <p className="leading-[1.6] mb-0 text-[15px]">​</p>
              <p className="leading-[1.6] mb-0 text-[15px]">
                If you wish to request the removal of a Review that you have
                submitted, you may contact the Company at the Support center.
              </p>
              <p className="leading-[1.6] mb-0 text-[15px]">​</p>
              <p className="font-serif font-semibold leading-[1.6] mb-[8px] text-[16px]">
                4.5 User Responsibilities and Rights
              </p>
              <p className="leading-[1.6] mb-0 text-[15px]">​</p>
              <p className="leading-[1.6] mb-0 text-[15px]">
                You retain ownership of any User Content you submit, subject to
                the license granted to the Company. By submitting User Content,
                you represent and warrant that you own or have all necessary
                rights and permissions to grant the above license. You confirm
                that your content does not infringe upon any third-party
                intellectual property, privacy, or contractual rights and that
                any third-party rights, including moral rights, in the User
                Content have been lawfully waived or granted to you. The Company
                and its successors may use the User Content without compensating
                you.
              </p>
              <p className="leading-[1.6] mb-0 text-[15px]">​</p>
              <p className="leading-[1.6] mb-0 text-[15px]">
                Once submitted, User Content may remain accessible even if
                removed by you. If you wish to revoke the granted license for
                specific User Content, you may contact the Support center.
                However, any prior use of the content will not be affected. The
                Company is not responsible for storing or maintaining copies of
                removed User Content and is not liable for any loss incurred due
                to its removal.
              </p>
              <p className="leading-[1.6] mb-0 text-[15px]">​</p>
              <p className="font-serif font-semibold leading-[1.6] mb-[8px] text-[16px]">
                4.6 Content Moderation and Restrictions
              </p>
              <p className="leading-[1.6] mb-0 text-[15px]">​</p>
              <p className="leading-[1.6] mb-0 text-[15px]">
                The Company does not actively monitor User Content but reserves
                the right to review, edit, remove, or restrict content at its
                discretion. This applies particularly to content that contains
                offensive, illegal, defamatory, or misleading material, violates
                third-party intellectual property, privacy, or contractual
                rights, promotes harm, violence, harassment, or illegal
                activity, or disrupts the functionality, security, or reputation
                of the Service. The Company assumes no liability for the
                accuracy or legality of User Content submitted by others.
              </p>
              <p className="leading-[1.6] mb-0 text-[15px]">​</p>
              <p className="font-serif font-semibold leading-[1.6] mb-[8px] text-[16px]">
                4.7 Prohibited Conduct
              </p>
              <p className="leading-[1.6] mb-0 text-[15px]">​</p>
              <p className="leading-[1.6] mb-0 text-[15px]">
                You agree not to use the Service to distribute illegal,
                deceptive, or harmful content, impersonate another individual or
                misrepresent your affiliation, reverse-engineer, decompile,
                disassemble, extract, or manipulate any part of the Service, or
                interfere with the security, availability, or integrity of the
                Service. Violation of these Terms may result in the immediate
                suspension or termination of your account, as well as legal
                consequences.
              </p>
              <p className="leading-[1.6] mb-0 text-[15px]">​</p>
              <p className="font-serif font-semibold leading-[1.6] mb-[8px] text-[16px]">
                4.8 Service Availability and Modifications
              </p>
              <p className="leading-[1.6] mb-0 text-[15px]">​</p>
              <p className="leading-[1.6] mb-0 text-[15px]">
                The Company reserves the right to modify, update, interrupt,
                suspend, or discontinue any aspect of the Service at any time
                without notice to you or our liability. You acknowledge that a
                variety of Company’s actions may impair or prevent you from
                accessing the Service at certain times and/or in the same way,
                for limited periods or permanently, and agree that the Company
                has no responsibility or liability as a result of any such
                actions or results, including, without limitation, for the
                deletion of, or failure to make available to you, any content or
                parts of the Services. Certain features may not be available in
                all regions or on all devices. If a modification impacts your
                use of the Service, you may cancel your subscription or delete
                your account.
              </p>
              <p className="leading-[1.6] mb-0 text-[15px]">​</p>
              <p className="leading-[1.6] mb-0 text-[15px]">
                You are solely responsible for obtaining the equipment and
                telecommunication services necessary to access the Service, and
                all fees associated with it (such as computing devices and
                Internet service provider and airtime charges).
              </p>
              <p className="leading-[1.6] mb-0 text-[15px]">​</p>
              <p className="font-serif font-semibold leading-[1.6] mb-[8px] text-[16px]">
                4.9 Risks and Disclaimer of Liability
              </p>
              <p className="leading-[1.6] mb-0 text-[15px]">​</p>
              <p className="leading-[1.6] mb-0 text-[15px]">
                Your use of the Service is at your own risk. The Company does
                not guarantee the accuracy, reliability, or fitness of any
                content provided. We are not responsible for loss of data,
                device malfunctions, or technical failures, any reliance on
                wellness or other recommendations within the Service, or
                personal injury, financial loss, or any legal claims arising
                from your use of the Service.
              </p>
              <p className="leading-[1.6] mb-0 text-[15px]">​</p>
              <p className="font-serif font-semibold leading-[1.6] mb-[8px] text-[16px]">
                4.10 Customer Support
              </p>
              <p className="leading-[1.6] mb-0 text-[15px]">​</p>
              <p className="leading-[1.6] text-[15px]">
                Customer support services are provided at the Company’s
                discretion. While we may assist users, there is no obligation to
                provide support or respond to inquiries. If you require
                assistance, contact the Support center, and we will respond as
                reasonably possible.
              </p>
            </div>
          </section>
          <section
            className="[word-break:break-word] content-stretch flex flex-col gap-[12px] items-start relative shrink-0 w-full figma-section"
            data-node-id="813:2382"
            data-name="Section / 5"
          >
            <h2
              className="font-display leading-[normal] not-italic relative shrink-0 text-[#1a1612] text-[16px] w-full"
              data-node-id="813:2383"
              id="third-party-services-materials-and-advertising"
            >
              5. THIRD-PARTY SERVICES, MATERIALS, AND ADVERTISING
            </h2>
            <div
              className="font-serif font-normal leading-[0] relative shrink-0 text-[#2d241e] text-[0px] w-full whitespace-pre-wrap"
              data-node-id="813:2384"
            >
              <p className="leading-[1.6] mb-0 text-[15px]">
                The Service may integrate, provide access to, or display content
                from third-party services, websites, software, advertisements,
                and other materials (“Third-Party Services” and “Third-Party
                Materials”). This includes external links, embedded content, and
                user-generated materials contributed by third parties. While
                these features may be accessible through the Service, the
                Company does not control or assume responsibility for the
                content, functionality, or policies of any Third-Party Services.
              </p>
              <p className="leading-[1.6] mb-0 text-[15px]">​</p>
              <p className="font-serif font-semibold leading-[1.6] mb-[8px] text-[16px]">
                5.1 No Endorsement or Responsibility
              </p>
              <p className="leading-[1.6] mb-0 text-[15px]">​</p>
              <p className="leading-[1.6] mb-0 text-[15px]">
                By using the Service, you acknowledge that the Company does not
                endorse, verify, or assume responsibility for the accuracy,
                legality, quality, or reliability of any Third-Party Services or
                Third-Party Materials. Some of this content may be
                objectionable, offensive, or misleading, and the Company is not
                liable for any exposure to such material. Any interactions,
                transactions, or agreements you engage in with third parties
                through the Service are solely between you and the respective
                third party. The Company bears no responsibility for any
                disputes, losses, or issues that may arise from these
                interactions.
              </p>
              <p className="leading-[1.6] mb-0 text-[15px]">​</p>
              <p className="font-serif font-semibold leading-[1.6] mb-[8px] text-[16px]">
                5.2 Third-Party Links and Advertising
              </p>
              <p className="leading-[1.6] mb-0 text-[15px]">​</p>
              <p className="leading-[1.6] mb-0 text-[15px]">
                The Service may include advertisements, sponsored content, or
                links to third-party websites that are not owned or controlled
                by the Company. Clicking on third-party links or engaging with
                external services does not establish any endorsement,
                affiliation, or sponsorship between the Company and the third
                party. Any engagement with such content is at your own risk. It
                is your responsibility to review and comply with the terms,
                policies, and privacy practices of third-party services before
                using them. The Company disclaims any liability for how third
                parties collect, process, or use your data.
              </p>
              <p className="leading-[1.6] mb-0 text-[15px]">​</p>
              <p className="font-serif font-semibold leading-[1.6] mb-[8px] text-[16px]">
                5.3 No Liability for Third-Party Content
              </p>
              <p className="leading-[1.6] mb-0 text-[15px]">​</p>
              <p className="leading-[1.6] mb-0 text-[15px]">
                The Company does not monitor, evaluate, or guarantee the
                accuracy, completeness, or legality of Third-Party Materials. To
                the fullest extent permitted by law, the Company makes no
                express or implied warranties regarding third-party content and
                disclaims all liability for any loss, damage, or harm resulting
                from your reliance on or use of such content. Some third-party
                materials may be outdated, misleading, or otherwise unreliable,
                and you assume full responsibility for any decisions based on
                this content.
              </p>
              <p className="leading-[1.6] mb-0 text-[15px]">​</p>
              <p className="font-serif font-semibold leading-[1.6] mb-[8px] text-[16px]">
                5.4 Use of Third-Party Services at Your Own Risk
              </p>
              <p className="leading-[1.6] mb-0 text-[15px]">​</p>
              <p className="leading-[1.6] mb-0 text-[15px]">
                Accessing Third-Party Services through the Service is entirely
                voluntary. You assume all risks associated with interacting with
                third-party content, including potential malware, phishing
                scams, or deceptive practices. The Company is not responsible
                for any technical issues, disputes, or damages arising from your
                engagement with Third-Party Services. By using such services,
                you waive any claims against the Company related to your
                interactions with third-party content, advertisements, or
                external links.
              </p>
              <p className="leading-[1.6] mb-0 text-[15px]">​</p>
              <p className="leading-[1.6] text-[15px]">
                If you encounter harmful, misleading, or offensive third-party
                content while using the Service, you may report it to the
                Company. However, the Company is not obligated to investigate,
                remove, or take action against third-party content unless
                required by law.
              </p>
            </div>
          </section>
          <section
            className="[word-break:break-word] content-stretch flex flex-col gap-[12px] items-start relative shrink-0 w-full figma-section"
            data-node-id="813:2385"
            data-name="Section / 6"
          >
            <h2
              className="font-display leading-[normal] not-italic relative shrink-0 text-[#1a1612] text-[16px] w-full"
              data-node-id="813:2386"
              id="subscription-fees-payments-and-refunds"
            >
              6. SUBSCRIPTION FEES, PAYMENTS, AND REFUNDS
            </h2>
            <div
              className="font-serif font-normal leading-[0] relative shrink-0 text-[#2d241e] text-[0px] w-full whitespace-pre-wrap"
              data-node-id="813:2387"
            >
              <p className="leading-[1.6] mb-0 text-[15px]">
                This Section governs all purchases made through the Service,
                including but not limited to subscriptions, balance top-ups, and
                one-time purchases.
              </p>
              <p className="leading-[1.6] mb-0 text-[15px]">​</p>
              <p className="font-serif font-semibold leading-[1.6] mb-[8px] text-[16px]">
                6.1 Balance Terms
              </p>
              <p className="leading-[1.6] mb-0 text-[15px]">​</p>
              <p className="font-serif font-semibold leading-[1.6] mb-[4px] text-[15px]">
                6.1.1 Account Balance
              </p>
              <p className="leading-[1.6] mb-0 text-[15px]">​</p>
              <p className="leading-[1.6] mb-0 text-[15px]">
                To access certain features of the Services, you may be required
                to top-up your account balance (the “Balance”) with balance
                units (the “Balance Units”). The denomination and form of your
                Balance Units will be determined by the specific offer terms
                applicable to your arrangement with the Company. Balance Units
                will be deducted from your Balance when you access paid features
                of the Services on a per-minute basis, with the applicable
                amount deducted at the beginning of each minute of use. The cost
                of the access per minute may vary; it depends on the offer terms
                you accepted and the rates of advisors (psychics) applicable at
                any given moment or situation. The Company discloses the
                applicable cost of the access per minute before allowing you to
                access the respective features of the Services.
              </p>
              <p className="leading-[1.6] mb-0 text-[15px]">​</p>
              <p className="font-serif font-semibold leading-[1.6] mb-[4px] text-[15px]">
                6.1.2 Auto-Refill
              </p>
              <p className="leading-[1.6] mb-0 text-[15px]">​</p>
              <p className="leading-[1.6] mb-0 text-[15px]">
                You may also use the auto-refill feature to use certain paid
                features of the Services and your readings without interruption.
                This feature automatically tops-up your Balance every time the
                Balance Units run off. You may expressly authorise us to
                automatically top-up your Balance with the package of the
                Balance Units you choose, whenever there are not enough Balance
                Units to pay for the next minute of accessing the feature, until
                you cancel.
              </p>
              <p className="leading-[1.6] mb-0 text-[15px]">​</p>
              <p className="font-serif font-semibold leading-[1.6] mb-[4px] text-[15px]">
                6.1.3 Payment Method
              </p>
              <p className="leading-[1.6] mb-0 text-[15px]">​</p>
              <p className="leading-[1.6] mb-0 text-[15px]">
                To top-up your Balance, we will charge the payment method you
                provided at the time of purchase. You authorise us to charge the
                applicable fees to the payment method that you provided. You
                will be notified whenever your Balance is topped-up.
              </p>
              <p className="leading-[1.6] mb-0 text-[15px]">​</p>
              <p className="font-serif font-semibold leading-[1.6] mb-[4px] text-[15px]">
                6.1.4 How to Cancel
              </p>
              <p className="leading-[1.6] mb-0 text-[15px]">​</p>
              <p className="leading-[1.6] mb-0 text-[15px]">
                To cancel the automatic topping-up of your Balance, you may
                deactivate the auto-refill feature via your account settings. If
                you cancel the automatic topping-up of the Balance, it will be
                disabled, but you will still be able to use all the Balance
                Units remaining on your Balance.
              </p>
              <p className="leading-[1.6] mb-0 text-[15px]">​</p>
              <p className="font-serif font-semibold leading-[1.6] mb-[4px] text-[15px]">
                6.1.5 Limitations
              </p>
              <p className="leading-[1.6] mb-0 text-[15px]">​</p>
              <p className="leading-[1.6] mb-0 text-[15px]">
                All Balance Units are subject to the following conditions:
              </p>
              <p className="leading-[1.6] mb-0 text-[15px]">​</p>
              <p className="leading-[1.6] mb-0 text-[15px]">
                • All Balance Units may be forfeited if your account is
                suspended or terminated for any reason, at our sole and absolute
                discretion without prior notice, including, without limitation,
                for the following reasons:
              </p>
              <p className="leading-[1.6] mb-0 text-[15px]">{`    • Your account is disabled due to inactivity;`}</p>
              <p className="leading-[1.6] mb-0 text-[15px]">{`    • You fail to comply with the Terms of Use, Privacy Policy, associated supplementary policies, or other applicable rules related to the use of the Services;`}</p>
              <p className="leading-[1.6] mb-0 text-[15px]">{`    • You or your account is involved in a fraudulent, suspicious, unlawful, or other sort of infringing activity with respect to the Balance Units or the Services;`}</p>
              <p className="leading-[1.6] mb-0 text-[15px]">{`    • We are acting to protect the Services, our users, advisors, or our reputation.`}</p>
              <p className="leading-[1.6] mb-0 text-[15px]">
                • You agree that the Balance Units have no monetary value, do
                not constitute actual currency, property, or a personal property
                right of any type, and have no value outside of the Services.
                The Balance Units are not electronic money of any kind, do not
                accrue interest, dividends, or other earnings, and are not
                insured by any governmental agency, guarantee fund, or
                compensation mechanism. All Balance Units shall be deemed fully
                and fairly earned by the Company immediately upon placement into
                your Balance.
              </p>
              <p className="leading-[1.6] mb-0 text-[15px]">
                • The Balance Units may never be sold, transferred, traded,
                exchanged, withdrawn, gifted, or otherwise assigned through any
                means other than those established by us, whether in exchange
                for legally acceptable money, goods, or other items of monetary
                value, or otherwise.
              </p>
              <p className="leading-[1.6] mb-0 text-[15px]">
                • You may not buy or sell any Balance Units or your account in
                exchange for legally acceptable money or otherwise exchange them
                for any other kind of value through any means other than that
                established by us.
              </p>
              <p className="leading-[1.6] mb-0 text-[15px]">​</p>
              <p className="leading-[1.6] mb-0 text-[15px]">
                Refunds. Unless otherwise specified under the Money-Back Policy
                or required by applicable law, the Balance Units are
                non-refundable.
              </p>
              <p className="leading-[1.6] mb-0 text-[15px]">​</p>
              <p className="font-serif font-semibold leading-[1.6] mb-[8px] text-[16px]">
                6.2 Subscription Terms
              </p>
              <p className="leading-[1.6] mb-0 text-[15px]">​</p>
              <p className="font-serif font-semibold leading-[1.6] mb-[4px] text-[15px]">
                6.2.1 Subscription Options, Purchases and Refunds
              </p>
              <p className="leading-[1.6] mb-0 text-[15px]">​</p>
              <p className="leading-[1.6] mb-0 text-[15px]">
                The Service offers subscription-based access to its certain
                features and content, which may be purchased either directly
                from the Company (“Direct Purchase”) or via an App Store
                (“In-App Purchase”). You will pay the applicable fees (and any
                related taxes) as they become due. You authorize us or the
                applicable App Store, as the case may be, to charge the
                applicable fees to the payment method that you submit. All
                applicable subscription fees, billing terms, and durations
                (e.g., weekly, monthly, quarterly, annually) will be displayed
                on the payment screen or at checkout before payment
                authorization. Our pricing varies based on a number of factors
                including, but not limited to, region, bundle, and duration of
                subscription. Some limited features of the Service may be
                available free of charge, but full access requires a paid
                subscription.
              </p>
              <p className="leading-[1.6] mb-0 text-[15px]">​</p>
              <p className="leading-[1.6] mb-0 text-[15px]">
                For information on refunds, see Refunds under General Terms
                below.
              </p>
              <p className="leading-[1.6] mb-0 text-[15px]">​</p>
              <p className="font-serif font-semibold leading-[1.6] mb-[4px] text-[15px]">
                6.2.2 Purchases and Payment Processing
              </p>
              <p className="leading-[1.6] mb-0 text-[15px]">​</p>
              <p className="leading-[1.6] mb-0 text-[15px]">
                Upon completing the onboarding process on the Website or in the
                App, you will be presented with available subscription options,
                their pricing, durations, and accepted payment methods (e.g.,
                Mastercard, Visa, PayPal, Apple Pay, Google Pay). By selecting a
                subscription and authorizing the payment, you instruct the
                applicable payment processor or App Store to charge your
                selected payment method. Once the payment is validated, you will
                receive access to the Service. For Direct Purchases, payments
                are handled by third-party payment processors, which you
                authorize to charge your selected payment method. These
                processors handle transaction processing and notify us of
                successful payments. For In-App Purchases, payments are
                processed by the App Store, and the respective App Store’s terms
                and policies govern the transaction.
              </p>
              <p className="leading-[1.6] mb-0 text-[15px]">​</p>
              <p className="font-serif font-semibold leading-[1.6] mb-[4px] text-[15px]">
                6.2.3 Auto-Renewal, Subscription Continuity and Cancellation
              </p>
              <p className="leading-[1.6] mb-0 text-[15px]">​</p>
              <p className="leading-[1.6] mb-0 text-[15px]">
                All subscriptions automatically renew unless canceled. The
                renewal period matches the initial subscription term unless
                otherwise disclosed at the time of purchase. To avoid renewal,
                you must cancel your subscription before the renewal date.
              </p>
              <p className="leading-[1.6] mb-0 text-[15px]">​</p>
              <p className="leading-[1.6] mb-0 text-[15px]">
                By proceeding with a subscription, you acknowledge that charges
                will be applied periodically based on the selected billing
                cycle. The renewal rate will be no more than the rate for the
                immediately prior subscription period, excluding any promotional
                (introductory) and discount pricing, unless we notify you of a
                rate change prior to your auto-renewal.
              </p>
              <p className="leading-[1.6] mb-0 text-[15px]">​</p>
              <p className="leading-[1.6] mb-0 text-[15px]">
                You authorize us and our payment processing providers to store
                your payment account information, including any updates, and use
                it in connection with your use of the Services as described in
                the applicable offer terms. We may verify your payment method
                with a small temporary charge, which will be refunded to you
                within ten (10) business days.
              </p>
              <p className="leading-[1.6] mb-0 text-[15px]">​</p>
              <p className="leading-[1.6] mb-0 text-[15px]">
                For Direct Purchases, cancellation must be completed through the
                Website’s account settings or by following the cancellation
                instructions provided at the time of purchase. For In-App
                Purchases, cancellation must be done through the respective App
                Store’s account settings. Deleting the App does not cancel your
                subscription.
              </p>
              <p className="leading-[1.6] mb-0 text-[15px]">​</p>
              <p className="leading-[1.6] mb-0 text-[15px]">
                For more details, check our article explaining the cancellation
                procedure. If you cancel, your access to the paid features of
                the Services will remain active until the end of the
                then-current subscription period, after which it will be
                disabled; however, you will retain access to all free features
                of the Services.
              </p>
              <p className="leading-[1.6] mb-0 text-[15px]">​</p>
              <p className="font-serif font-semibold leading-[1.6] mb-[4px] text-[15px]">
                6.2.4 Add-On Items and Additional Services
              </p>
              <p className="leading-[1.6] mb-0 text-[15px]">​</p>
              <p className="leading-[1.6] mb-0 text-[15px]">
                In addition to your subscription, you may have the option to
                purchase add-on items such as premium content, consultations, or
                supplementary features. These add-ons may be one-time purchases
                or recurring charges. Canceling your main subscription will also
                cancel any associated recurring add-ons, but canceling an add-on
                alone does not affect your primary subscription.
              </p>
              <p className="leading-[1.6] mb-0 text-[15px]">​</p>
              <p className="font-serif font-semibold leading-[1.6] mb-[4px] text-[15px]">
                6.2.5 Free Trials and Promotional Offers
              </p>
              <p className="leading-[1.6] mb-0 text-[15px]">​</p>
              <p className="leading-[1.6] mb-0 text-[15px]">
                We may offer a free trial or introductory offer subscription for
                the Service according to the terms specified when you sign up
                for the offer. A free trial provides you access to the Service
                for a period of time; an introductory offer provides you access
                to the Service at a reduced rate for a limited period of time.
                Unless you cancel before the end of the free trial or
                introductory offer, or unless otherwise stated, your access to
                the Service will automatically continue and you will be billed
                the applicable fees. It is your responsibility to know when the
                free trial or introductory offer will end. We reserve the right,
                in our absolute discretion, to modify or terminate any free
                trial or introductory offer, your access to the Service during
                the free trial or introductory period, or any of these terms
                without notice and with no liability. We reserve the right to
                limit your ability to take advantage of multiple free trials
                and/or introductory offers.
              </p>
              <p className="leading-[1.6] mb-0 text-[15px]">​</p>
              <p className="font-serif font-semibold leading-[1.6] mb-[4px] text-[15px]">
                6.2.6 Promotional Codes
              </p>
              <p className="leading-[1.6] mb-0 text-[15px]">​</p>
              <p className="leading-[1.6] mb-0 text-[15px]">
                We may provide you with gift cards or promotional codes that can
                be redeemed for additional features, enhancements,
                functionalities, content, services within a specified Service
                and for a limited period of time, subject eligibility
                requirements (the “Promotional Codes”). Promotional Codes have
                no cash value, are personal, non-transferable,
                non-sublicensable, and we are under no obligation to provide any
                compensation in connection with a Promotional Code.
              </p>
              <p className="leading-[1.6] mb-0 text-[15px]">​</p>
              <p className="font-serif font-semibold leading-[1.6] mb-[4px] text-[15px]">
                6.2.7 Changes to Subscription Fees
              </p>
              <p className="leading-[1.6] mb-0 text-[15px]">​</p>
              <p className="leading-[1.6] mb-0 text-[15px]">
                To the maximum extent permitted by applicable law, we may modify
                subscription fees at any time. If notification is required under
                applicable law, we will inform you of pricing changes in the
                manner and timeframe mandated by regulations. Where no specific
                timeframe is prescribed, we will provide notice by posting the
                updated prices in the App or on our website, sending an email
                notification, or using other prominent communication methods.
                The revised pricing will take effect as specified in the notice.
                If you do not agree to the updated fees, you may cancel your
                subscription before the new pricing takes effect or refrain from
                prepaying for continued access to the Service.
              </p>
              <p className="leading-[1.6] mb-0 text-[15px]">​</p>
              <p className="font-serif font-semibold leading-[1.6] mb-[4px] text-[15px]">
                6.2.8 Failure to Pay and Service Termination
              </p>
              <p className="leading-[1.6] mb-0 text-[15px]">​</p>
              <p className="leading-[1.6] mb-0 text-[15px]">
                If a payment is declined or not received when due, we may notify
                you to update your payment method. However, if the issue is not
                resolved, we reserve the right to suspend or terminate your
                access to the Service without further notice. Any content, data,
                or personalized settings associated with your account may be
                lost, and we are not responsible for restoring them.
              </p>
              <p className="leading-[1.6] mb-0 text-[15px]">​</p>
              <p className="font-serif font-semibold leading-[1.6] mb-[8px] text-[16px]">
                6.3 General Terms
              </p>
              <p className="leading-[1.6] mb-0 text-[15px]">​</p>
              <p className="font-serif font-semibold leading-[1.6] mb-[4px] text-[15px]">
                6.3.1 Refunds
              </p>
              <p className="leading-[1.6] mb-0 text-[15px]">​</p>
              <p className="leading-[1.6] mb-0 text-[15px]">
                Unless otherwise specified under the Money-Back Policy or
                required by applicable law, all purchases are non-refundable. To
                determine your eligibility for a refund and obtain relevant
                instructions, please refer to our Money-Back Policy.
              </p>
              <p className="leading-[1.6] mb-0 text-[15px]">​</p>
              <p className="font-serif font-semibold leading-[1.6] mb-[4px] text-[15px]">
                6.3.2 Chargebacks and Payment Disputes
              </p>
              <p className="leading-[1.6] mb-0 text-[15px]">​</p>
              <p className="leading-[1.6] mb-0 text-[15px]">
                If you wish to request a refund for a payment made using a
                reimbursable method, such as a credit or debit card, we strongly
                encourage you to contact us first at our Support center before
                initiating a chargeback with your payment provider. This allows
                us the opportunity to review your request and attempt to resolve
                the issue directly.
              </p>
              <p className="leading-[1.6] mb-0 text-[15px]">​</p>
              <p className="leading-[1.6] mb-0 text-[15px]">
                Refunds, when applicable, are not processed in real-time. If we
                confirm that a refund has been issued, please allow at least 15
                business days for the refund to reflect in your account. You
                acknowledge that you are not entitled to receive multiple
                refunds for the same transaction and agree that if you request a
                refund directly from us, you will not initiate a separate refund
                request or chargeback through your payment provider unless your
                request has been denied by us. If you receive duplicate refunds
                due to separate refund requests, we reserve the right to work
                with your payment provider to reverse one of those refunds.
              </p>
              <p className="leading-[1.6] mb-0 text-[15px]">​</p>
              <p className="leading-[1.6] mb-0 text-[15px]">
                Initiating a chargeback or reversing a payment through your bank
                or payment provider may result in the immediate termination of
                your account at our sole discretion, as it indicates that you
                have determined you no longer wish to use our Service. If a
                chargeback is overturned in our favor, you may contact the
                Support center to discuss reinstating your account.
              </p>
              <p className="leading-[1.6] mb-0 text-[15px]">​</p>
              <p className="leading-[1.6] mb-0 text-[15px]">
                As outlined in our Privacy Policy, your personally identifiable
                information may be shared with our payment processor to assist
                in responding to chargeback requests.
              </p>
              <p className="leading-[1.6] mb-0 text-[15px]">​</p>
              <p className="leading-[1.6] mb-0 text-[15px]">
                If you initiate a chargeback or payment dispute, we may suspend
                or terminate your access to the Service. Fraudulent or improper
                chargebacks may result in a permanent ban from using the Service
                and potential legal action.
              </p>
              <p className="leading-[1.6] mb-0 text-[15px]">​</p>
              <p className="font-serif font-semibold leading-[1.6] mb-[4px] text-[15px]">
                6.3.3 Taxes
              </p>
              <p className="leading-[1.6] mb-0 text-[15px]">​</p>
              <p className="leading-[1.6] text-[15px]">
                You agree to pay all taxes and similar assessments, including
                sales tax, use tax, value-added tax (VAT), and goods and
                services tax (GST), imposed by any government on your Services.
                If we do not collect taxes at the time of original purchase, we
                reserve the right to collect such taxes later using the same
                payment method with written notice explaining such charges. We
                also reserve the right to collect any penalties or interest
                imposed on your transactions if they are your fault (for
                example, if you provide us with a false address or tax status at
                the time of purchase).
              </p>
            </div>
          </section>
          <section
            className="[word-break:break-word] content-stretch flex flex-col gap-[12px] items-start relative shrink-0 w-full figma-section"
            data-node-id="813:2388"
            data-name="Section / 7"
          >
            <h2
              className="font-display leading-[normal] not-italic relative shrink-0 text-[#1a1612] text-[16px] w-full"
              data-node-id="813:2389"
              id="user-representations-and-restrictions"
            >
              7. USER REPRESENTATIONS AND RESTRICTIONS
            </h2>
            <div
              className="font-serif font-normal leading-[0] relative shrink-0 text-[#2d241e] text-[0px] w-full whitespace-pre-wrap"
              data-node-id="813:2390"
            >
              <p className="leading-[1.6] mb-0 text-[15px]">
                By accessing or using the Service, you confirm that:
              </p>
              <p className="leading-[1.6] mb-0 text-[15px]">​</p>
              <p className="leading-[1.6] mb-0 text-[15px]">
                • You have the legal capacity to enter into and comply with
                these Terms.
              </p>
              <p className="leading-[1.6] mb-0 text-[15px]">
                • You are at least 18 years old and legally permitted to use the
                Service.
              </p>
              <p className="leading-[1.6] mb-0 text-[15px]">
                • You will not access the Service through automated or non-human
                means, including bots, scripts, or similar methods.
              </p>
              <p className="leading-[1.6] mb-0 text-[15px]">
                • You will not use the Service for any unlawful, fraudulent, or
                unauthorized purpose.
              </p>
              <p className="leading-[1.6] mb-0 text-[15px]">
                • You are not located in a country subject to U.S. government
                embargo restrictions or designated as a terrorist-supporting
                nation.
              </p>
              <p className="leading-[1.6] mb-0 text-[15px]">
                • You are not listed on any U.S. government list of prohibited
                or restricted persons.
              </p>
              <p className="leading-[1.6] mb-0 text-[15px]">
                • Your use of the Service complies with all applicable laws and
                regulations.
              </p>
              <p className="leading-[1.6] mb-0 text-[15px]">​</p>
              <p className="leading-[1.6] mb-0 text-[15px]">
                If any information you provide is false, misleading, outdated,
                or incomplete, we reserve the right to deny or terminate your
                current or future access to the Service.
              </p>
              <p className="leading-[1.6] mb-0 text-[15px]">​</p>
              <p className="font-serif font-semibold leading-[1.6] mb-[8px] text-[16px]">
                7.1 Permitted Use of the Service
              </p>
              <p className="leading-[1.6] mb-0 text-[15px]">​</p>
              <p className="leading-[1.6] mb-0 text-[15px]">
                The Service is made available for its intended purposes only.
                You may not use the Service for any unauthorized, commercial, or
                competitive activities unless expressly approved by us.
              </p>
              <p className="leading-[1.6] mb-0 text-[15px]">​</p>
              <p className="font-serif font-semibold leading-[1.6] mb-[8px] text-[16px]">
                7.2 Prohibited Conduct
              </p>
              <p className="leading-[1.6] mb-0 text-[15px]">​</p>
              <p className="leading-[1.6] mb-0 text-[15px]">
                You agree not to engage in the following activities when using
                the Service:
              </p>
              <p className="leading-[1.6] mb-0 text-[15px]">​</p>
              <p className="leading-[1.6] mb-0 text-[15px]">
                • Collecting, scraping, or systematically retrieving data or
                other content from the Service to create a database,
                compilation, or directory without our express permission.
              </p>
              <p className="leading-[1.6] mb-0 text-[15px]">
                • Using the Service for any unauthorized purposes, including
                modifying, adapting, improving, or creating derivative works
                from the Service.
              </p>
              <p className="leading-[1.6] mb-0 text-[15px]">
                • Using the Service for commercial or revenue-generating
                endeavors, unless explicitly approved by us.
              </p>
              <p className="leading-[1.6] mb-0 text-[15px]">
                • Making the Service accessible over a network that allows
                multiple devices or users to access it simultaneously, unless
                permitted.
              </p>
              <p className="leading-[1.6] mb-0 text-[15px]">
                • Developing, launching, or using the Service to create a
                competing product, service, or software.
              </p>
              <p className="leading-[1.6] mb-0 text-[15px]">
                • Using any of our proprietary information, intellectual
                property, or interfaces to develop, license, or distribute
                applications, accessories, or other related products.
              </p>
              <p className="leading-[1.6] mb-0 text-[15px]">
                • Tampering with any notices or technological restrictions in
                the Service.
              </p>
              <p className="leading-[1.6] mb-0 text-[15px]">
                • Using the Service to host, transmit, or otherwise aid illegal,
                abusive (including unsolicited), fraudulent, deceptive,
                threatening, explicit, obscene, hateful, or harmful content or
                behavior or malicious code;
              </p>
              <p className="leading-[1.6] mb-0 text-[15px]">
                • Misrepresenting yourself or impersonating another person.
              </p>
              <p className="leading-[1.6] mb-0 text-[15px]">
                • Circumventing, disabling, or interfering with security
                features of the Service.
              </p>
              <p className="leading-[1.6] mb-0 text-[15px]">
                • Framing, embedding, or linking to the Service without
                authorization.
              </p>
              <p className="leading-[1.6] mb-0 text-[15px]">
                • Interfering with or disrupting the Service, networks, or
                servers connected to the Service, or creating an undue burden on
                our infrastructure.
              </p>
              <p className="leading-[1.6] mb-0 text-[15px]">
                • Decompiling, disassembling, reverse-engineering, or otherwise
                attempting to access the source code of any part of the Service.
              </p>
              <p className="leading-[1.6] mb-0 text-[15px]">
                • Bypassing or attempting to bypass access restrictions or
                security measures implemented in the Service.
              </p>
              <p className="leading-[1.6] mb-0 text-[15px]">
                • Uploading, transmitting, or distributing malware, viruses,
                worms, trojans, or other harmful software that could damage the
                Service or others’ devices.
              </p>
              <p className="leading-[1.6] mb-0 text-[15px]">
                • Using, launching, or distributing any automated system (e.g.,
                bots, spiders, scrapers, cheat utilities) to access or interact
                with the Service.
              </p>
              <p className="leading-[1.6] mb-0 text-[15px]">
                • Sending unsolicited commercial emails or engaging in
                spam-related activities.
              </p>
              <p className="leading-[1.6] mb-0 text-[15px]">
                • Engaging in any activity that may harm, tarnish, or damage the
                reputation of the Company or the Service.
              </p>
              <p className="leading-[1.6] mb-0 text-[15px]">
                • Using the Service in violation of any applicable laws or
                regulations.
              </p>
              <p className="leading-[1.6] mb-0 text-[15px]">{`• Attempting any of the above or otherwise infringing these Terms or other applicable policies and rules, including rules from Trust & Safety Center.`}</p>
              <p className="leading-[1.6] mb-0 text-[15px]">​</p>
              <p className="leading-[1.6] mb-0 text-[15px]">
                If we reasonably determine that you violate any of the use
                restrictions above, we may suspend or terminate your access to
                the Service or utilize other mechanisms available to prevent
                violations, including removing violating content or users. We
                will exercise commercially reasonable efforts to give you notice
                without unreasonable delay after taking protective action.
              </p>
              <p className="leading-[1.6] mb-0 text-[15px]">​</p>
              <p className="font-serif font-semibold leading-[1.6] mb-[8px] text-[16px]">
                7.3 Respectful Conduct Towards Customer Support
              </p>
              <p className="leading-[1.6] mb-0 text-[15px]">​</p>
              <p className="leading-[1.6] text-[15px]">
                We expect all users to interact with our customer support team
                in a respectful and professional manner. If at any time your
                communication or behavior is deemed harassing, abusive,
                threatening, or offensive, we reserve the right to terminate
                your account immediately.
              </p>
            </div>
          </section>
          <section
            className="[word-break:break-word] content-stretch flex flex-col gap-[12px] items-start relative shrink-0 w-full figma-section"
            data-node-id="813:2391"
            data-name="Section / 8"
          >
            <h2
              className="font-display leading-[normal] not-italic relative shrink-0 text-[#1a1612] text-[16px] w-full"
              data-node-id="813:2392"
              id="disclaimer-of-warranties"
            >
              8. DISCLAIMER OF WARRANTIES
            </h2>
            <div
              className="font-serif font-normal leading-[0] relative shrink-0 text-[#2d241e] text-[0px] w-full whitespace-pre-wrap"
              data-node-id="813:2393"
            >
              <p className="font-serif font-semibold leading-[1.6] mb-[8px] text-[16px]">
                8.1 General Disclaimers
              </p>
              <p className="leading-[1.6] mb-0 text-[15px]">​</p>
              <p className="leading-[1.6] mb-0 text-[15px]">
                EXCEPT TO THE EXTENT PROHIBITED BY LAW OR OTHERWISE
                INAPPLICABLE, YOU EXPRESSLY ACKNOWLEDGE AND AGREE THAT YOUR USE
                OF THE SERVICE IS AT YOUR OWN RISK. THE SERVICE AND ANY PRODUCTS
                OR CONTENT PROVIDED THROUGH IT ARE MADE AVAILABLE “AS IS” AND
                “AS AVAILABLE,” WITHOUT ANY WARRANTIES OR GUARANTEES OF ANY
                KIND, EXPRESS OR IMPLIED.
              </p>
              <p className="leading-[1.6] mb-0 text-[15px]">​</p>
              <p className="leading-[1.6] mb-0 text-[15px]">
                TO THE FULLEST EXTENT PERMITTED BY LAW, THE COMPANY AND ITS
                AFFILIATES, OFFICERS, EMPLOYEES, AGENTS, PARTNERS, AND LICENSORS
                EXPRESSLY DISCLAIM ALL WARRANTIES, WHETHER EXPRESS, IMPLIED, OR
                STATUTORY, INCLUDING BUT NOT LIMITED TO IMPLIED WARRANTIES OF
                MERCHANTABILITY, FITNESS FOR A PARTICULAR PURPOSE,
                NON-INFRINGEMENT, ACCURACY, AND RELIABILITY OF CONTENT OR DATA.
              </p>
              <p className="leading-[1.6] mb-0 text-[15px]">​</p>
              <p className="leading-[1.6] mb-0 text-[15px]">
                IN PARTICULAR, WE DO NOT WARRANT THAT:
              </p>
              <p className="leading-[1.6] mb-0 text-[15px]">​</p>
              <p className="leading-[1.6] mb-0 text-[15px]">
                • The Service will meet your expectations or requirements;
              </p>
              <p className="leading-[1.6] mb-0 text-[15px]">
                • The Service will be uninterrupted, secure, error-free, or free
                from technical issues;
              </p>
              <p className="leading-[1.6] mb-0 text-[15px]">
                • The results obtained from using the Service will be accurate,
                reliable, or error-free;
              </p>
              <p className="leading-[1.6] mb-0 text-[15px]">
                • The quality of any content, features, or services will meet
                your expectations;
              </p>
              <p className="leading-[1.6] mb-0 text-[15px]">
                • Any defects or errors will be corrected promptly or at all.
              </p>
              <p className="leading-[1.6] mb-0 text-[15px]">​</p>
              <p className="leading-[1.6] mb-0 text-[15px]">
                ANY MATERIAL, DATA, OR INFORMATION OBTAINED THROUGH THE SERVICE
                IS ACCESSED AT YOUR OWN DISCRETION AND RISK. YOU ARE SOLELY
                RESPONSIBLE FOR ANY DAMAGE TO YOUR DEVICE OR LOSS OF DATA
                RESULTING FROM YOUR USE OF THE SERVICE.
              </p>
              <p className="leading-[1.6] mb-0 text-[15px]">​</p>
              <p className="leading-[1.6] mb-0 text-[15px]">
                WE DO NOT GUARANTEE OR PROMISE ANY SPECIFIC RESULTS FROM USING
                THE SERVICE. BY USING THE SERVICE, YOU ACCEPT THE INHERENT RISKS
                OF SERVICE INTERRUPTIONS, TECHNICAL FAILURES, AND POTENTIAL DATA
                LOSS.
              </p>
              <p className="leading-[1.6] mb-0 text-[15px]">​</p>
              <p className="font-serif font-semibold leading-[1.6] mb-[8px] text-[16px]">
                8.2 Service Modifications and Updates
              </p>
              <p className="leading-[1.6] mb-0 text-[15px]">​</p>
              <p className="leading-[1.6] mb-0 text-[15px]">
                We reserve the right to update, modify, or discontinue any
                aspect of the Service, including features, content, and
                availability, at any time, with or without notice. This includes
                changes to:
              </p>
              <p className="leading-[1.6] mb-0 text-[15px]">​</p>
              <p className="leading-[1.6] mb-0 text-[15px]">
                • The information provided on our website and mobile
                applications;
              </p>
              <p className="leading-[1.6] mb-0 text-[15px]">
                • The availability or functionality of any Service feature;
              </p>
              <p className="leading-[1.6] mb-0 text-[15px]">
                • The pricing, structure, or terms of use of the Service.
              </p>
              <p className="leading-[1.6] mb-0 text-[15px]">​</p>
              <p className="leading-[1.6] mb-0 text-[15px]">
                We are not responsible for any loss or inconvenience resulting
                from modifications, suspensions, or discontinuations of any part
                of the Service.
              </p>
              <p className="leading-[1.6] mb-0 text-[15px]">​</p>
              <p className="font-serif font-semibold leading-[1.6] mb-[8px] text-[16px]">
                8.3 Consumer Protection and Non-Waivable Rights
              </p>
              <p className="leading-[1.6] mb-0 text-[15px]">​</p>
              <p className="leading-[1.6] text-[15px]">
                Nothing in these Terms shall exclude or limit any consumer
                rights that cannot be waived under applicable law. If you are
                entitled to statutory rights under the laws of your country of
                residence, those rights remain unaffected by these disclaimers.
              </p>
            </div>
          </section>
          <section
            className="[word-break:break-word] content-stretch flex flex-col gap-[12px] items-start relative shrink-0 w-full figma-section"
            data-node-id="813:2394"
            data-name="Section / 9"
          >
            <h2
              className="font-display leading-[normal] not-italic relative shrink-0 text-[#1a1612] text-[16px] w-full"
              data-node-id="813:2395"
              id="limitation-of-liability"
            >
              9. LIMITATION OF LIABILITY
            </h2>
            <div
              className="font-serif font-normal leading-[0] relative shrink-0 text-[#2d241e] text-[0px] w-full whitespace-pre-wrap"
              data-node-id="813:2396"
            >
              <p className="leading-[1.6] mb-0 text-[15px]">
                TO THE MAXIMUM EXTENT PERMITTED BY LAW, WE (INCLUDING OUR
                AFFILIATES, OFFICERS, EMPLOYEES, AGENTS, PARTNERS, AND
                LICENSORS) SHALL NOT BE LIABLE TO YOU OR ANY THIRD PARTY FOR ANY
                INDIRECT, INCIDENTAL, CONSEQUENTIAL, EXEMPLARY, SPECIAL, OR
                PUNITIVE DAMAGES, INCLUDING BUT NOT LIMITED TO LOST PROFITS,
                LOST DATA, BUSINESS INTERRUPTION, OR ANY OTHER LOSSES ARISING
                FROM YOUR USE OF OR INABILITY TO USE THE SERVICE, PRODUCTS, OR
                ANY THIRD-PARTY ADS, EVEN IF WE HAVE BEEN ADVISED OF THE
                POSSIBILITY OF SUCH DAMAGES.
              </p>
              <p className="leading-[1.6] mb-0 text-[15px]">​</p>
              <p className="leading-[1.6] mb-0 text-[15px]">
                YOUR ACCESS TO AND USE OF THE SERVICE (INCLUDING THE APP,
                CONTENT, AND USER CONTENT) AND THIRD-PARTY ADS ARE AT YOUR OWN
                RISK. YOU AGREE THAT YOU ARE SOLELY RESPONSIBLE FOR ANY DAMAGE
                TO YOUR DEVICE, LOSS OF DATA, OR OTHER HARM THAT RESULTS FROM
                YOUR USE OF THE SERVICE.
              </p>
              <p className="leading-[1.6] mb-0 text-[15px]">​</p>
              <p className="font-serif font-semibold leading-[1.6] mb-[8px] text-[16px]">
                9.1 Limitation of Aggregate Liability
              </p>
              <p className="leading-[1.6] mb-0 text-[15px]">​</p>
              <p className="leading-[1.6] mb-0 text-[15px]">
                NOTWITHSTANDING ANYTHING TO THE CONTRARY HEREIN, OUR TOTAL
                LIABILITY TO YOU FOR ANY CLAIMS ARISING OUT OF OR RELATED TO
                YOUR USE OF THE SERVICE, PRODUCTS, OR CONTENT SHALL BE LIMITED
                TO THE TOTAL AMOUNT PAID BY YOU TO US FOR ACCESS TO THE SERVICE
                DURING THE TWELVE (12) MONTHS IMMEDIATELY PRECEDING THE EVENT
                GIVING RISE TO THE CLAIM, OR IF GREATER, ONE HUNDRED EURO (€
                100).
              </p>
              <p className="leading-[1.6] mb-0 text-[15px]">​</p>
              <p className="leading-[1.6] mb-0 text-[15px]">
                THESE LIMITATIONS OF LIABILITY FORM A FUNDAMENTAL BASIS OF THE
                AGREEMENT BETWEEN YOU AND THE COMPANY. WITHOUT THESE
                LIMITATIONS, WE WOULD NOT BE ABLE TO OFFER THE SERVICE UNDER THE
                SAME TERMS.
              </p>
              <p className="leading-[1.6] mb-0 text-[15px]">​</p>
              <p className="font-serif font-semibold leading-[1.6] mb-[8px] text-[16px]">
                9.2 Waiver of Unknown Claims (California Residents)
              </p>
              <p className="leading-[1.6] mb-0 text-[15px]">​</p>
              <p className="leading-[1.6] mb-0 text-[15px]">
                IF YOU ARE A RESIDENT OF CALIFORNIA, YOU EXPRESSLY WAIVE
                CALIFORNIA CIVIL CODE SECTION 1542, WHICH STATES:
              </p>
              <p className="leading-[1.6] mb-0 text-[15px]">​</p>
              <p className="leading-[1.6] mb-0 text-[15px]">
                “A GENERAL RELEASE DOES NOT EXTEND TO CLAIMS THAT THE CREDITOR
                OR RELEASING PARTY DOES NOT KNOW OR SUSPECT TO EXIST IN HIS OR
                HER FAVOR AT THE TIME OF EXECUTING THE RELEASE, AND THAT, IF
                KNOWN BY HIM OR HER, WOULD HAVE MATERIALLY AFFECTED HIS OR HER
                SETTLEMENT WITH THE DEBTOR OR RELEASED PARTY.”
              </p>
              <p className="leading-[1.6] mb-0 text-[15px]">​</p>
              <p className="leading-[1.6] mb-0 text-[15px]">
                BY ACCEPTING THESE TERMS, YOU RECOGNIZE AND AGREE THAT YOU MAY
                BE WAIVING RIGHTS WITH RESPECT TO CLAIMS THAT ARE CURRENTLY
                UNKNOWN OR UNSUSPECTED.
              </p>
              <p className="leading-[1.6] mb-0 text-[15px]">​</p>
              <p className="font-serif font-semibold leading-[1.6] mb-[8px] text-[16px]">
                9.3 Jurisdiction-Specific Exceptions
              </p>
              <p className="leading-[1.6] mb-0 text-[15px]">​</p>
              <p className="leading-[1.6] mb-0 text-[15px]">
                SOME JURISDICTIONS DO NOT ALLOW CERTAIN LIMITATIONS OR
                EXCLUSIONS OF LIABILITY, INCLUDING FOR INCIDENTAL OR
                CONSEQUENTIAL DAMAGES. TO THE EXTENT THAT ANY PART OF THESE
                LIMITATIONS IS FOUND TO BE UNENFORCEABLE UNDER APPLICABLE LAW,
                THE REMAINING LIMITATIONS SHALL STILL APPLY TO THE MAXIMUM
                EXTENT PERMITTED.
              </p>
              <p className="leading-[1.6] mb-0 text-[15px]">​</p>
              <p className="leading-[1.6] text-[15px]">
                IF ANY REMEDY SET FORTH IN THESE TERMS IS FOUND TO HAVE FAILED
                ITS ESSENTIAL PURPOSE, ALL REMAINING LIMITATIONS OF LIABILITY
                SHALL STILL APPLY. ADDITIONAL CONSUMER RIGHTS MAY APPLY
                DEPENDING ON YOUR JURISDICTION.
              </p>
            </div>
          </section>
          <section
            className="[word-break:break-word] content-stretch flex flex-col gap-[12px] items-start relative shrink-0 w-full figma-section"
            data-node-id="813:2397"
            data-name="Section / 10"
          >
            <h2
              className="font-display leading-[normal] not-italic relative shrink-0 text-[#1a1612] text-[16px] w-full"
              data-node-id="813:2398"
              id="indemnification"
            >
              10. INDEMNIFICATION
            </h2>
            <div
              className="font-serif font-normal leading-[0] relative shrink-0 text-[#2d241e] text-[15px] w-full whitespace-pre-wrap"
              data-node-id="813:2399"
            >
              <p className="leading-[1.6] mb-0">
                You agree to defend, indemnify, and hold harmless the Company,
                along with its affiliates, parent companies, officers,
                employees, agents, partners, licensors, contractors, successors,
                and assigns (each, an “Indemnitee”), from and against any
                losses, damages, liabilities, claims, demands, judgments,
                settlements, penalties, fines, costs, and expenses of any
                kind—including, but not limited to, reasonable attorneys’ fees
                and professional fees—arising directly or indirectly from:
              </p>
              <p className="leading-[1.6] mb-0">​</p>
              <p className="leading-[1.6] mb-0">
                • User Content, including any claims that such content infringes
                upon third-party rights or violates applicable laws.
              </p>
              <p className="leading-[1.6] mb-0">
                • Your breach of these Terms, whether by you or anyone using
                your account or device.
              </p>
              <p className="leading-[1.6] mb-0">
                • Your access to or use of the Service, including any actions
                taken on the platform.
              </p>
              <p className="leading-[1.6] mb-0">
                • Your violation of any applicable law, regulation, or
                third-party rights, including intellectual property, privacy, or
                proprietary rights.
              </p>
              <p className="leading-[1.6] mb-0">
                • Any claims related to property damage, personal injury, bodily
                harm, or death resulting from your use of the Service in
                violation of these Terms.
              </p>
              <p className="leading-[1.6] mb-0">​</p>
              <p className="leading-[1.6]">
                The Company reserves the right to assume full control of the
                defense, negotiation, and settlement of any claim for which you
                are required to indemnify us. You agree to fully cooperate with
                our defense efforts and acknowledge that we have the sole
                discretion to select legal counsel and strategy in such matters.
                You may not settle any claim that imposes liability or
                obligations on the Company without our prior written consent.
              </p>
            </div>
          </section>
          <section
            className="[word-break:break-word] content-stretch flex flex-col gap-[12px] items-start relative shrink-0 w-full figma-section"
            data-node-id="813:2400"
            data-name="Section / 11"
          >
            <h2
              className="font-display leading-[normal] not-italic relative shrink-0 text-[#1a1612] text-[16px] w-full"
              data-node-id="813:2401"
              id="international-use"
            >
              11. INTERNATIONAL USE
            </h2>
            <p
              className="font-serif font-normal leading-[1.6] relative shrink-0 text-[#2d241e] text-[15px] w-full"
              data-node-id="813:2402"
            >
              The Company makes no representation that the Service is
              accessible, appropriate or legally available for use in your
              jurisdiction, and accessing and using the Service is prohibited
              from territories where doing so would be illegal. You access the
              Service at your own initiative and are responsible for compliance
              with local laws.
            </p>
          </section>
          <section
            className="[word-break:break-word] content-stretch flex flex-col gap-[12px] items-start relative shrink-0 w-full figma-section"
            data-node-id="813:2403"
            data-name="Section / 12"
          >
            <h2
              className="font-display leading-[normal] not-italic relative shrink-0 text-[#1a1612] text-[16px] w-full"
              data-node-id="813:2404"
              id="informal-dispute-resolution-procedures-and-arbitration"
            >
              12. INFORMAL DISPUTE RESOLUTION PROCEDURES AND ARBITRATION
            </h2>
            <div
              className="font-serif font-normal leading-[0] relative shrink-0 text-[#2d241e] text-[0px] w-full whitespace-pre-wrap"
              data-node-id="813:2405"
            >
              <p className="leading-[1.6] mb-0 text-[15px]">
                PLEASE READ THIS PROVISION CAREFULLY TO ENSURE THAT YOU
                UNDERSTAND—THIS SECTION CONTROLS HOW DISPUTES BETWEEN YOU AND
                THE COMPANY WILL BE ADDRESSED.
              </p>
              <p className="leading-[1.6] mb-0 text-[15px]">​</p>
              <p className="leading-[1.6] mb-0 text-[15px]">
                BY AGREEING TO THIS PROVISION, YOU ARE WAIVING YOUR RIGHT TO
                PARTICIPATE IN A CLASS ACTION LAWSUIT AND YOU ARE WAIVING YOUR
                RIGHT TO A JURY TRIAL.
              </p>
              <p className="leading-[1.6] mb-0 text-[15px]">​</p>
              <p className="leading-[1.6] mb-0 text-[15px]">
                YOU ARE ALSO AGREEING TO RESOLVE ALL DISPUTES BETWEEN YOU AND
                THE COMPANY THROUGH BINDING ARBITRATION, UNLESS YOU EXERCISE
                YOUR RIGHT TO REJECT ARBITRATION AS PROVIDED BELOW.
              </p>
              <p className="leading-[1.6] mb-0 text-[15px]">​</p>
              <p className="leading-[1.6] mb-0 text-[15px]">
                You and Spiritual Staryield Limited (“we” or the “Company”)
                agree to resolve all Disputes through binding arbitration, as
                described below, except for: (i) claims that fall within the
                jurisdiction of a small claims court, provided such claims are
                not class action disputes and also meet the court’s
                jurisdictional and monetary limits; and (ii) disputes related to
                intellectual property rights. A “Dispute” means any claim,
                controversy, or legal action—whether arising from past, present,
                or future events, and based on contract, tort, statute, or
                common law—between you and the Company regarding the Website,
                Services, or this agreement (the “Arbitration Agreement”).
                “Dispute” also includes disputes about the interpretation,
                applicability, or enforceability of these terms or the formation
                of this Arbitration Agreement, including whether any part of it
                is invalid or unenforceable.
              </p>
              <p className="leading-[1.6] mb-0 text-[15px]">​</p>
              <p className="font-serif font-semibold leading-[1.6] mb-[8px] text-[16px]">
                12.1 Mandatory Pre-Filing Notice Procedure
              </p>
              <p className="leading-[1.6] mb-0 text-[15px]">​</p>
              <p className="leading-[1.6] mb-0 text-[15px]">
                You and we agree that good faith, informal efforts to resolve
                disputes often result in a faster and less expensive outcome.
                Therefore, if you intend to assert a claim for any Dispute (as
                defined above) against the Company, you must first send the
                Company a written notice of the Dispute (“Notice”) that gives
                the Company some basic information about you and the Dispute.
                Any Notice must include (i) your name, address, and email
                address, (ii) a detailed description of your Dispute; (iii) any
                relevant facts regarding your use of the Website and Services
                (including your account ID, profile screenshots, and anything
                else that will help us identify your account); (iv) a detailed
                description of the relief you are seeking, including a
                calculation of any money damages you are seeking; and (v) a
                personally signed statement from you (and not your attorney)
                verifying the accuracy of the information in Notice. The Notice
                must be individualized, meaning it can concern only your dispute
                and no other person’s dispute. If you are filling out a Notice
                for another person, you must include all information described
                above, and also a statement describing your relationship to the
                person and why the person is unable to fill out the Notice for
                themselves.
              </p>
              <p className="leading-[1.6] mb-0 text-[15px]">​</p>
              <p className="leading-[1.6] mb-0 text-[15px]">
                You must send the Notice to the Company at the following
                address:
              </p>
              <p className="leading-[1.6] mb-0 text-[15px]">​</p>
              <p className="leading-[1.6] mb-0 text-[15px]">{`62 Athalassas Avenue, Mezzanine Floor, Strovolos, 2012 Nicosia, Cyprus      Attention: Legal team`}</p>
              <p className="leading-[1.6] mb-0 text-[15px]">​</p>
              <p className="leading-[1.6] mb-0 text-[15px]">
                If we need to send you a Notice, we will send the Notice to you
                at the contact information we have available for you, which may
                include, if applicable, the contact information associated with
                your account.
              </p>
              <p className="leading-[1.6] mb-0 text-[15px]">​</p>
              <p className="leading-[1.6] mb-0 text-[15px]">
                After we receive a Notice, you and we agree to engage in good
                faith efforts to resolve the Dispute between us for a period of
                60 days through informal negotiation. The 60-day period can be
                extended if you and we agree that such an extension is likely to
                lead to a resolution. As part of the informal negotiation
                process, you and we agree that we will both attend at least one
                individualized video conference (“Video Conference”). The Video
                Conference can be via Zoom, Microsoft Teams, WhatsApp, or any
                other similar platform that you and we agree on and that we both
                have access to. The Video Conference can be held after the
                60-day period, if necessary. If you are represented by an
                attorney in your Dispute, your attorney may participate in the
                Video Conference, but you are still required to attend and
                participate in good faith. The Company is also required to
                participate in the Video Conference by sending one or more of
                its representatives, and the Company may also send one or more
                of its attorneys. If you are unable to participate in the Video
                Conference by video, you may attend telephonically if you
                certify in writing that circumstances exist that prevent you
                from appearing by video (such as your lack of access to a phone
                with a working camera or your inability to connect to a stable
                internet connection). You and we agree that we (and our
                attorneys, if represented) shall work cooperatively to schedule
                the Video Conference at the earliest mutually convenient time
                after we receive a Notice. You and we also agree to use our best
                efforts to resolve the Dispute at the Video Conference. If you
                and we cannot resolve the issues identified in the Notice within
                60 days after the completed Notice is received (or a longer time
                if agreed), you or we may commence an arbitration proceeding or
                a small claims court proceeding.
              </p>
              <p className="leading-[1.6] mb-0 text-[15px]">​</p>
              <p className="leading-[1.6] mb-0 text-[15px]">
                Compliance with these Informal Dispute Resolution Procedures is
                Mandatory and Pre-Filing Notice procedures (including the Video
                Conference requirement) are a condition precedent to initiating
                any arbitration or small claims court action. Failure to follow
                the procedures is a breach of this Arbitration Agreement.
              </p>
              <p className="leading-[1.6] mb-0 text-[15px]">​</p>
              <p className="leading-[1.6] mb-0 text-[15px]">
                The Mandatory Pre-Filing Notice procedures are essential so that
                you and the Company have a meaningful opportunity to resolve
                Disputes in an inexpensive and efficient manner. Unless
                prohibited by applicable law, the arbitration provider shall not
                accept or administer any demand for arbitration unless the party
                bringing the demand for arbitration certifies in writing that
                the Mandatory Pre-Filing Notice procedures (including the Video
                Conference requirement) were fully satisfied. If the party
                bringing the demand for arbitration fails to include a written
                certification that the Pre-Filing Notice procedures (including
                the Video Conference) were met, then the arbitration forum shall
                administratively close the demand for arbitration and no fees
                shall be due from the responding party. A court of competent
                jurisdiction shall have authority to enforce this provision and
                to enjoin any arbitration proceeding or small claims court
                action accordingly.
              </p>
              <p className="leading-[1.6] mb-0 text-[15px]">​</p>
              <p className="leading-[1.6] mb-0 text-[15px]">
                All offers, promises, conduct, and statements made in the course
                of the Mandatory Pre-Filing Notice process by any party, its
                agents, employees, and attorneys are confidential and not
                admissible for any purpose in any subsequent proceeding (except
                as required to certify in writing that the Mandatory Pre-Filing
                Notice procedures were completed before submitting a demand for
                arbitration). Evidence that is otherwise admissible or
                discoverable shall not be rendered inadmissible or
                non-discoverable by this section.
              </p>
              <p className="leading-[1.6] mb-0 text-[15px]">​</p>
              <p className="font-serif font-semibold leading-[1.6] mb-[8px] text-[16px]">
                12.2 Small Claims Court
              </p>
              <p className="leading-[1.6] mb-0 text-[15px]">​</p>
              <p className="leading-[1.6] mb-0 text-[15px]">
                Subject to applicable jurisdictional requirements and the
                Mandatory Pre-Filing Notice requirements explained above, you or
                the Company may elect to pursue a Dispute in a local small
                claims court rather than through arbitration, so long as the
                matter remains in small claims court and proceeds only on an
                individual basis. If a party has already submitted an
                arbitration demand, the other party may, in its sole discretion,
                inform the arbitral forum that it chooses to have the Dispute
                heard in small claims court. At that time, the arbitral forum
                will administratively close the arbitration and the Dispute will
                be heard in the appropriate small claims court, with no fees due
                from the arbitration respondent.
              </p>
              <p className="leading-[1.6] mb-0 text-[15px]">​</p>
              <p className="font-serif font-semibold leading-[1.6] mb-[8px] text-[16px]">
                12.3 What is Arbitration?
              </p>
              <p className="leading-[1.6] mb-0 text-[15px]">​</p>
              <p className="leading-[1.6] mb-0 text-[15px]">
                Arbitration is a more informal way to resolve our disagreements
                than a lawsuit in court. For instance, arbitration uses a
                neutral arbitrator instead of a judge or jury, generally
                involves more limited discovery, when permissible, and is
                subject to very limited review by courts. Although the process
                is more informal, arbitrators can award some of the same
                individualized damages and relief that a court can award. An
                arbitrator cannot, however, order a party to act or stop doing
                something—this is known as “equitable relief.” Either you or we
                can go to court and seek equitable relief, including by filing a
                motion to compel the other party to follow this Arbitration
                Agreement. However, you and we agree that the only courts where
                we will seek equitable relief are the state and federal courts
                in Delaware. This exception for equitable relief does not waive
                this Arbitration Agreement. You and we agree that the U.S.
                Federal Arbitration Act and federal arbitration law govern the
                interpretation and enforcement of this provision. A court of
                competent jurisdiction has exclusive authority to resolve any
                dispute relating to the interpretation, applicability, or
                enforceability of this binding arbitration agreement. This
                arbitration provision shall survive termination of these terms
                and the termination of your account.
              </p>
              <p className="leading-[1.6] mb-0 text-[15px]">​</p>
              <p className="font-serif font-semibold leading-[1.6] mb-[8px] text-[16px]">
                12.4 CLASS ACTION AND JURY TRIAL WAIVER
              </p>
              <p className="leading-[1.6] mb-0 text-[15px]">​</p>
              <p className="leading-[1.6] mb-0 text-[15px]">
                TO THE FULLEST EXTENT ALLOWABLE BY LAW, YOU AND THE COMPANY
                WAIVE THE RIGHT TO A JURY TRIAL AND THE RIGHT TO LITIGATE
                DISPUTES IN COURT IN FAVOR OF ARBITRATION (EXCEPT FOR SMALL
                CLAIMS COURT DESCRIBED ABOVE). YOU AND THE COMPANY EACH WAIVE
                THE RIGHT TO FILE OR PARTICIPATE IN A CLASS ACTION LAWSUIT
                AGAINST THE OTHER, INCLUDING ANY CURRENTLY PENDING ACTIONS
                AGAINST THE COMPANY. TO THE FULLEST EXTENT ALLOWABLE BY LAW,
                THERE SHALL BE NO RIGHT OR AUTHORITY FOR ANY CLAIMS TO BE
                LITIGATED IN COURT ON A CLASS, COLLECTIVE, REPRESENTATIVE, OR
                CONSOLIDATED BASIS.
              </p>
              <p className="leading-[1.6] mb-0 text-[15px]">​</p>
              <p className="leading-[1.6] mb-0 text-[15px]">
                EXCEPT FOR THE MASS FILING PROCEDURES DESCRIBED BELOW, YOU AND
                WE AGREE THAT
              </p>
              <p className="leading-[1.6] mb-0 text-[15px]">​</p>
              <p className="leading-[1.6] mb-0 text-[15px]">
                • THE ARBITRATOR MAY ONLY AWARD FINAL RELIEF IN FAVOR OF THE
                INDIVIDUAL PARTY SEEKING RELIEF AND ONLY TO THE EXTENT NECESSARY
                TO PROVIDE FINAL RELIEF WARRANTED BY THAT INDIVIDUAL PARTY’S
                CLAIM.
              </p>
              <p className="leading-[1.6] mb-0 text-[15px]">
                • THE ARBITRATOR MAY NOT AWARD FINAL RELIEF FOR, AGAINST, OR ON
                BEHALF OF ANYONE WHO IS NOT A PARTY TO THE ARBITRATION ON A
                CLASS, COLLECTIVE, OR REPRESENTATIVE BASIS.
              </p>
              <p className="leading-[1.6] mb-0 text-[15px]">​</p>
              <p className="leading-[1.6] mb-0 text-[15px]">
                IF A COURT DETERMINES THAT ANY OF THE PROHIBITIONS IN THIS
                PARAGRAPH ARE UNENFORCEABLE FOR A PARTICULAR CLAIM OR REQUEST
                FOR RELIEF, AND ALL APPEALS OF THAT DECISION ARE AFFIRMED AND
                SUCH DECISION BECOMES FINAL, THEN YOU AND THE COMPANY AGREE THAT
                PARTICULAR CLAIM OR REQUEST FOR RELIEF SHALL PROCEED IN COURT
                BUT SHALL BE STAYED PENDING INDIVIDUAL ARBITRATION OF THE
                REMAINING CLAIMS FOR RELIEF THAT YOU HAVE BROUGHT.
              </p>
              <p className="leading-[1.6] mb-0 text-[15px]">​</p>
              <p className="font-serif font-semibold leading-[1.6] mb-[8px] text-[16px]">
                12.5 Arbitration Procedure
              </p>
              <p className="leading-[1.6] mb-0 text-[15px]">​</p>
              <p className="leading-[1.6] mb-0 text-[15px]">{`The arbitration will be governed by applicable rules of National Arbitration & Mediation (“NAM”) (including the Comprehensive Dispute Resolution Rules and Procedures and the Supplemental Rules for Mass Arbitration Filings, as applicable) (“NAM Rules”), as modified by this Arbitration Agreement, and will be administered by NAM. The NAM Rules are available online at www.namadr.com or by requesting them in writing at the Notice address listed above. You may obtain a form to initiate arbitration with NAM at https://www.namadr.com/content/uploads/2024/03/Comprehensive-Demand-for-Arb-revised-3.21.2024.pdf or by contacting NAM.`}</p>
              <p className="leading-[1.6] mb-0 text-[15px]">​</p>
              <p className="leading-[1.6] mb-0 text-[15px]">
                If NAM is unavailable or unwilling to do so, another arbitration
                provider shall be selected by the parties for that purpose, or
                if the parties are unable to agree on an alternative
                administrator, by the court pursuant to 9 U.S.C. §5.
              </p>
              <p className="leading-[1.6] mb-0 text-[15px]">​</p>
              <p className="leading-[1.6] mb-0 text-[15px]">
                You and we agree that the party initiating arbitration must
                submit a written certification that they have complied with and
                completed the Mandatory Pre-Filing Notice and Informal Dispute
                Resolution Procedures requirements enclosed with any demand for
                arbitration. The demand for arbitration and certification must
                be personally signed by the party initiating arbitration (and
                their attorney, if represented).
              </p>
              <p className="leading-[1.6] mb-0 text-[15px]">​</p>
              <p className="leading-[1.6] mb-0 text-[15px]">
                The arbitration will be in English. A single independent and
                impartial arbitrator will be appointed remotely pursuant to the
                NAM Rules, as modified herein. You and the Company agree to
                comply with the following rules, which are intended to
                streamline the dispute resolution process and reduce the costs
                and burdens on the parties: (i) the arbitration will be
                conducted online and/or be solely based on written submissions,
                the specific manner to be chosen by the party initiating the
                arbitration; (ii) the arbitration will not require any personal
                appearance by the parties or witnesses unless otherwise mutually
                agreed in writing by the parties or the arbitrator decides that
                a formal hearing is necessary; and (iii) any judgment on the
                award the arbitrator renders may be entered in any court of
                competent jurisdiction.
              </p>
              <p className="leading-[1.6] mb-0 text-[15px]">​</p>
              <p className="leading-[1.6] mb-0 text-[15px]">
                If an in-person hearing is required and you reside in the United
                States, the hearing will take place in Delaware, unless the
                arbitrator determines that this would pose a hardship for you,
                in which case the in-person hearing may be conducted in the
                claimant’s state and county of residence or unless the laws of
                the state where your claim arose prohibit an out-of-state venue,
                in which case the arbitrator will determine the hearing venue in
                that state. If you reside outside the United States, the site of
                any in-person hearing will be determined by the NAM Rules.
              </p>
              <p className="leading-[1.6] mb-0 text-[15px]">​</p>
              <p className="leading-[1.6] mb-0 text-[15px]">
                The award of the arbitrator will be in writing and will include
                a statement setting forth the reasons for the disposition of any
                claim. The arbitrator will apply the laws of the Republic of
                Cyprus in conducting the arbitration, except when the
                substantive laws of the state where your claim arose prohibit
                doing so, in which case the arbitrator will apply the
                substantive law of the state where your claim arose. You
                acknowledge that these terms and your use of the Services
                evidence a transaction involving interstate commerce. The United
                States Federal Arbitration Act will govern the interpretation,
                enforcement, and proceedings.
              </p>
              <p className="leading-[1.6] mb-0 text-[15px]">​</p>
              <p className="leading-[1.6] mb-0 text-[15px]">
                The Arbitrator is bound by and shall adhere to this Arbitration
                Agreement. In the event NAM Rules conflict with this Arbitration
                Agreement, the terms of this Arbitration Agreement shall
                control. If the Arbitrator determines that strict application of
                any term of this Arbitration Agreement would result in a
                fundamentally unfair arbitration, then the Arbitrator shall have
                the authority to modify such term to the extent necessary to
                ensure a fundamentally fair arbitration that is consistent with
                efficient and inexpensive resolution of Disputes.
              </p>
              <p className="leading-[1.6] mb-0 text-[15px]">​</p>
              <p className="leading-[1.6] mb-0 text-[15px]">
                Unless you and the Company otherwise agree, the arbitration will
                be conducted virtually via video or teleconference.
              </p>
              <p className="leading-[1.6] mb-0 text-[15px]">​</p>
              <p className="font-serif font-semibold leading-[1.6] mb-[8px] text-[16px]">
                12.6 Decision of the Arbitrator
              </p>
              <p className="leading-[1.6] mb-0 text-[15px]">​</p>
              <p className="leading-[1.6] mb-0 text-[15px]">
                Barring extraordinary circumstances, the arbitrator will issue
                their decision within 120 days from the date the arbitrator is
                appointed. The arbitrator may extend this time limit for an
                additional 30 days in the interests of justice. All arbitration
                proceedings will be closed to the public and confidential, and
                all records relating thereto will be permanently sealed, except
                as necessary to obtain court confirmation of the arbitration
                award. The award of the arbitrator will be in writing and will
                include a statement setting forth the reasons for the
                disposition of any claim.
              </p>
              <p className="leading-[1.6] mb-0 text-[15px]">​</p>
              <p className="leading-[1.6] mb-0 text-[15px]">
                The arbitration award is binding only between you and the
                Company and will not have any preclusive effect in another
                arbitration or proceeding that involves a different party.
              </p>
              <p className="leading-[1.6] mb-0 text-[15px]">​</p>
              <p className="font-serif font-semibold leading-[1.6] mb-[8px] text-[16px]">
                12.7 Fees
              </p>
              <p className="leading-[1.6] mb-0 text-[15px]">​</p>
              <p className="leading-[1.6] mb-0 text-[15px]">
                The payment of arbitration fees (the fees imposed by the
                arbitration administrator, including filing, arbitrator, and
                hearing fees) will be governed by the applicable NAM Rules,
                unless you qualify for a fee waiver under applicable law. If
                after exhausting any potentially available fee waivers, the
                arbitrator finds that the arbitration fees will be prohibitive
                for you as compared to litigation, we will pay as much of your
                filing, arbitrator, and hearing fees in the arbitration as the
                arbitrator deems necessary to prevent the arbitration from being
                cost-prohibitive, regardless of the outcome of the arbitration,
                unless the arbitrator determines that your claim(s) were
                frivolous or brought for an improper purpose or asserted in bad
                faith.
              </p>
              <p className="leading-[1.6] mb-0 text-[15px]">​</p>
              <p className="leading-[1.6] mb-0 text-[15px]">
                You and we agree that arbitration should be cost-effective for
                all parties and that any party may engage with NAM to address
                the reduction or deferral of fees.
              </p>
              <p className="leading-[1.6] mb-0 text-[15px]">​</p>
              <p className="font-serif font-semibold leading-[1.6] mb-[8px] text-[16px]">
                12.8 Confidentiality
              </p>
              <p className="leading-[1.6] mb-0 text-[15px]">​</p>
              <p className="leading-[1.6] mb-0 text-[15px]">
                Upon either your or our request, the Arbitrator will issue an
                order requiring that confidential information of either party
                disclosed during the arbitration (whether in documents or
                orally) may not be used or disclosed except in connection with
                the arbitration or a proceeding to enforce the arbitration
                award, and that any permitted court filing of confidential
                information must be done under seal.
              </p>
              <p className="leading-[1.6] mb-0 text-[15px]">​</p>
              <p className="font-serif font-semibold leading-[1.6] mb-[8px] text-[16px]">
                12.9 Settlement Offers and Offers of Judgment
              </p>
              <p className="leading-[1.6] mb-0 text-[15px]">​</p>
              <p className="leading-[1.6] mb-0 text-[15px]">
                At least ten (10) calendar days before the date set for the
                arbitration hearing, you or the Company may serve a written
                offer of judgment upon the other party to allow judgment on
                specified terms. If the offer is accepted, the offer with proof
                of acceptance shall be submitted to the arbitration provider,
                who shall enter judgment accordingly. If the offer is not
                accepted prior to the arbitration hearing or within thirty (30)
                calendar days after it is made, whichever occurs first, it shall
                be deemed withdrawn, and cannot be given as evidence in the
                arbitration. If an offer made by one party is not accepted by
                the other party, and the other party fails to obtain a more
                favorable award, the other party shall not recover their
                post-offer costs and shall pay the offering party’s costs from
                the time of the offer (which, solely for purposes of offers of
                judgment, may include reasonable attorneys’ fees to the extent
                they are recoverable by statute, in an amount not to exceed the
                damages awarded).
              </p>
              <p className="leading-[1.6] mb-0 text-[15px]">​</p>
              <p className="leading-[1.6] mb-0 text-[15px]">
                The parties agree that any disputes with respect to settlement
                offer(s) or offer(s) of judgment in a Mass Filing are to be
                resolved by a single arbitrator to the extent such offers
                contain the same material terms. For arbitrations involving
                represented parties, the represented parties’ attorneys agree to
                communicate individual settlement offer(s) or offer(s) of
                judgment to each and every arbitration claimant or respondent to
                whom such offers are extended.
              </p>
              <p className="leading-[1.6] mb-0 text-[15px]">​</p>
              <p className="font-serif font-semibold leading-[1.6] mb-[8px] text-[16px]">
                12.10 Additional Procedures for Mass Arbitration Filings
              </p>
              <p className="leading-[1.6] mb-0 text-[15px]">​</p>
              <p className="leading-[1.6] mb-0 text-[15px]">
                The following provisions set forth additional procedures that
                apply to mass arbitration filings. If ten (10) or more similar
                claims are asserted against the Company by the same or
                coordinated attorneys or are otherwise coordinated, consistent
                with the definition and criteria of “Mass Filings” set forth in
                the NAM Rules, you and we understand and agree that these
                additional procedures shall apply and the resolution of your
                dispute might be delayed. You and we agree that throughout this
                process, our attorneys shall meet and confer to discuss
                modifications to these procedures based on the particular needs
                of the Mass Filing. You and we agree to make all reasonable
                efforts to maximize the integrity and efficiency of arbitration
                to resolve Disputes between us, particularly those involving
                Mass Filings, and further commit to acting in good faith to
                adhere to the procedures established in this section. The
                parties further agree that application of these Mass Filing
                procedures has been reasonably designed to result in an
                efficient and fair adjudication of claims.
              </p>
              <p className="leading-[1.6] mb-0 text-[15px]">​</p>
              <p className="leading-[1.6] mb-0 text-[15px]">
                Bellwether Arbitrations for Mass Filings. Bellwether proceedings
                are encouraged by courts and arbitration administrators where
                there are multiple disputes involving similar claims against the
                same or related parties. The parties shall select ten individual
                arbitration claims (five per side), designated as the “Initial
                Test Cases,” to proceed to arbitration. Only the Initial Test
                Cases shall be filed with the arbitrator. All other claims shall
                be held in abeyance. This means that the filing fees will be
                paid only for the Initial Test Cases; for all other demands for
                arbitration in a Mass Filing, the filing fees (together with any
                arbitrator consideration of the other demands) will be held in
                abeyance, and neither you nor the Company will be required to
                pay any such filing fees. You and the Company also agree that
                neither you nor we shall be deemed to be in breach of this
                Arbitration Agreement for failure to pay any such filing fees,
                and that neither you nor we shall be entitled to any
                contractual, statutory, or other remedies, damages, or sanctions
                of any kind for failure to pay any such filing fees. If,
                pursuant to this subsection, a party files non-Bellwether
                Arbitrations with the arbitration provider, the parties agree
                that the arbitration provider shall hold those demands in
                abeyance and not refer them to the arbitrator pending resolution
                of the Initial Test Cases. Unless the claims are resolved in
                advance or the schedule is extended, the arbitrators will render
                a final award for the Initial Test Cases within 120 days of the
                initial pre-hearing conference.
              </p>
              <p className="leading-[1.6] mb-0 text-[15px]">​</p>
              <p className="leading-[1.6] mb-0 text-[15px]">
                Global Mediation in Mass Filings. Following the resolution of
                the Initial Test Cases, the parties agree to engage in a global
                mediation of all the remaining individual arbitration claims
                comprising the Mass Filing (“Global Mediation”), deferring any
                filing costs associated with the non-Initial Test Cases until
                the Initial Test Cases and subsequent Global Mediation have
                concluded. After the final awards are provided to the mediator
                in the Initial Test Cases, the mediator and the parties shall
                have 90 days to agree upon a substantive methodology and make an
                offer to resolve the outstanding cases. If the Parties are
                unable to resolve the outstanding claims during the Global
                Mediation, the Parties may choose to opt out of the arbitration
                process and proceed in court with the remaining claims. Notice
                of the opt-out shall be provided in writing within 60 days of
                the close of the Global Mediation. Absent notice of an opt-out,
                the arbitrations may then be filed and administered by the
                arbitration provider. You and we also acknowledge that any
                applicable statute of limitations shall be tolled pending
                resolution of the global mediation process.
              </p>
              <p className="leading-[1.6] mb-0 text-[15px]">​</p>
              <p className="leading-[1.6] mb-0 text-[15px]">
                Severability. If any part of this Mass Arbitration provision is
                declared invalid, void, or unenforceable, then that provision is
                severable from the Arbitration Agreement and shall not affect
                the validity and enforceability of the remaining provisions.
              </p>
              <p className="leading-[1.6] mb-0 text-[15px]">​</p>
              <p className="font-serif font-semibold leading-[1.6] mb-[8px] text-[16px]">
                12.11 Opting Out of this Arbitration Agreement
              </p>
              <p className="leading-[1.6] mb-0 text-[15px]">​</p>
              <p className="leading-[1.6] mb-0 text-[15px]">
                Existing Users. Users who previously agreed to arbitrate may
                reject this updated Arbitration Agreement by following the
                opt-out method below, but such users will still be bound by the
                most recent prior version of the Arbitration Agreement and will
                otherwise be bound by these terms. Previous or existing users
                who do not opt out of this updated Arbitration Agreement will be
                bound by this Arbitration Agreement and it shall apply to all
                disputes between such users and the Company, including those
                arising (but not actually filed in arbitration) before the
                effective date of these terms. Arbitration demands that have
                already been filed with an arbitration provider before the
                effective date of this Arbitration Agreement and in compliance
                with a prior version of this Arbitration Agreement are subject
                to the prior version’s terms.
              </p>
              <p className="leading-[1.6] mb-0 text-[15px]">​</p>
              <p className="leading-[1.6] mb-0 text-[15px]">
                New Users. Users who first agree to these Terms—including this
                Arbitration Agreement—on or after April 2, 2026 may opt out of
                this Arbitration Agreement.
              </p>
              <p className="leading-[1.6] mb-0 text-[15px]">​</p>
              <p className="leading-[1.6] mb-0 text-[15px]">
                Method and Impact of Opting Out. Subject to the above, you may
                opt out of this Arbitration Agreement by sending written notice
                of your decision to opt out via this request form within 31 days
                from the later of the following dates: 1) the date you first use
                or attempt to use the Services, or 2) the date the Arbitration
                Agreement became effective as indicated in the “Last Updated”
                date of the terms, whichever is later. Your notice must include:
              </p>
              <p className="leading-[1.6] mb-0 text-[15px]">​</p>
              <p className="leading-[1.6] mb-0 text-[15px]">• Your name</p>
              <p className="leading-[1.6] mb-0 text-[15px]">
                • Your username (if any)
              </p>
              <p className="leading-[1.6] mb-0 text-[15px]">
                • The email address you used to set up your account (if you have
                one)
              </p>
              <p className="leading-[1.6] mb-0 text-[15px]">
                • An unequivocal statement that you want to opt out of this
                Arbitration Agreement
              </p>
              <p className="leading-[1.6] mb-0 text-[15px]">​</p>
              <p className="leading-[1.6] mb-0 text-[15px]">
                If you opt out of this Arbitration Agreement, all other parts of
                the terms and any other agreements between you and the Company
                will continue to apply to you. Opting out of this Arbitration
                Agreement has no effect on any other arbitration agreements that
                you may currently have, or may enter in the future, with us.
              </p>
              <p className="leading-[1.6] mb-0 text-[15px]">​</p>
              <p className="leading-[1.6] mb-0 text-[15px]">
                Arbitration Agreement Survival. This Arbitration Agreement will
                survive the termination of your relationship with the Company,
                including any revocation of consent or other action by you to
                end your participation in the Services or any communication with
                the Company.
              </p>
              <p className="leading-[1.6] mb-0 text-[15px]">​</p>
              <p className="leading-[1.6] mb-0 text-[15px]">
                Severability. If any portion of this Arbitration Agreement is
                found to be void, invalid, or otherwise unenforceable, then that
                portion shall be deemed to be severable and, if possible,
                superseded by a valid, enforceable provision, or portion
                thereof, that matches the intent of the original provision, or
                portion thereof, as closely as possible. The remainder of this
                Arbitration Agreement shall continue to be enforceable and valid
                according to the terms contained herein.
              </p>
              <p className="leading-[1.6] mb-0 text-[15px]">​</p>
              <p className="font-serif font-semibold leading-[1.6] mb-[8px] text-[16px]">
                12.12 Governing Law
              </p>
              <p className="leading-[1.6] mb-0 text-[15px]">​</p>
              <p className="leading-[1.6] text-[15px]">
                The laws of the Republic of Cyprus, excluding its conflicts of
                law rules, govern this Agreement and your use of the Service,
                except when the substantive laws of the state where your claim
                arose prohibit doing so. In that situation, the arbitrator will
                apply the substantive law of the state where your claim arose.
                Your use of the Services may also be subject to other local,
                state, national, or international laws. To the extent that any
                action relating to any dispute hereunder shall be brought in a
                court of law, such action will be subject to the exclusive
                jurisdiction of the state and federal courts located in
                Delaware, and you hereby irrevocably submit to personal
                jurisdiction in such courts, and waive any defense of
                inconvenient forum
              </p>
            </div>
          </section>
          <section
            className="[word-break:break-word] content-stretch flex flex-col gap-[12px] items-start relative shrink-0 w-full figma-section"
            data-node-id="813:2406"
            data-name="Section / 13"
          >
            <h2
              className="font-display leading-[normal] not-italic relative shrink-0 text-[#1a1612] text-[16px] w-full"
              data-node-id="813:2407"
              id="for-eea-and-uk-residents"
            >
              13. FOR EEA AND UK RESIDENTS
            </h2>
            <div
              className="font-serif font-normal leading-[0] relative shrink-0 text-[#2d241e] text-[15px] w-full whitespace-pre-wrap"
              data-node-id="813:2408"
            >
              <p className="leading-[1.6] mb-0">
                Nothing in these Terms shall deprive you of the consumer
                protection rights granted by the mandatory laws of your country
                of residence.
              </p>
              <p className="leading-[1.6] mb-0">​</p>
              <p className="leading-[1.6] mb-0">
                If you have a complaint, please contact us via this request
                form. The Company does not participate in any alternative
                dispute resolution scheme, except as required by law.
              </p>
              <p className="leading-[1.6] mb-0">​</p>
              <p className="leading-[1.6] mb-0">
                If a dispute arises under these Terms, you may bring legal
                proceedings before the competent courts of your habitual
                residence in the EEA or UK, and these courts shall have
                exclusive jurisdiction over the dispute. The Company shall also
                submit any disputes to the courts in your country of habitual
                residence.
              </p>
              <p className="leading-[1.6] mb-0">​</p>
              <p className="leading-[1.6]">
                These Terms, the Service, and any dispute between you and the
                Company shall be governed by the laws of England and Wales,
                excluding its conflict of law provisions. The 1980 UN Convention
                on Contracts for the International Sale of Goods shall not
                apply.
              </p>
            </div>
          </section>
          <section
            className="[word-break:break-word] content-stretch flex flex-col gap-[12px] items-start relative shrink-0 w-full figma-section"
            data-node-id="813:2409"
            data-name="Section / 14"
          >
            <h2
              className="font-display leading-[normal] not-italic relative shrink-0 text-[#1a1612] text-[16px] w-full"
              data-node-id="813:2410"
              id="for-california-residents"
            >
              14. FOR CALIFORNIA RESIDENTS
            </h2>
            <p
              className="font-serif font-normal leading-[1.6] relative shrink-0 text-[#2d241e] text-[15px] w-full"
              data-node-id="813:2411"
            >
              If you are a California resident, in accordance with Cal. Civ.
              Code § 1789.3, you may report complaints to the Complaint
              Assistance Unit of the Division of Consumer Services of the
              California Department of Consumer Affairs by contacting them in
              writing at 1625 North Market Blvd., Suite N 112 Sacramento, CA
              95834, or by telephone at (800) 952-5210.
            </p>
          </section>
          <section
            className="[word-break:break-word] content-stretch flex flex-col gap-[12px] items-start relative shrink-0 w-full figma-section"
            data-node-id="813:2412"
            data-name="Section / 15"
          >
            <h2
              className="font-display leading-[normal] not-italic relative shrink-0 text-[#1a1612] text-[16px] w-full"
              data-node-id="813:2413"
              id="limitation-on-claims-period"
            >
              15. LIMITATION ON CLAIMS PERIOD
            </h2>
            <div
              className="font-serif font-normal leading-[0] relative shrink-0 text-[#2d241e] text-[15px] w-full whitespace-pre-wrap"
              data-node-id="813:2414"
            >
              <p className="leading-[1.6] mb-0">
                You agree that regardless of any statute or law to the contrary
                or any applicable dispute resolution process, any claim or cause
                of action arising from or related to the use of the Services or
                these Terms must be filed within one (1) year from the date the
                claim or cause of action first arose. Failure to do so will
                result in your claim being permanently barred.
              </p>
              <p className="leading-[1.6] mb-0">​</p>
              <p className="leading-[1.6]">
                The provisions of this section, titled “Limitation on Claims
                Period”, constitute a separate legally binding agreement between
                you and the Company.
              </p>
            </div>
          </section>
          <section
            className="[word-break:break-word] content-stretch flex flex-col gap-[12px] items-start relative shrink-0 w-full figma-section"
            data-node-id="813:2415"
            data-name="Section / 16"
          >
            <h2
              className="font-display leading-[normal] not-italic relative shrink-0 text-[#1a1612] text-[16px] w-full"
              data-node-id="813:2416"
              id="miscellaneous"
            >
              16. MISCELLANEOUS
            </h2>
            <div
              className="font-serif font-normal leading-[0] relative shrink-0 text-[#2d241e] text-[15px] w-full whitespace-pre-wrap"
              data-node-id="813:2417"
            >
              <p className="leading-[1.6] mb-0">
                No failure or delay by the Company in exercising any of its
                rights under these Terms shall be deemed a waiver of such
                rights, nor shall any partial exercise of rights prevent the
                further enforcement of those or any other rights under these
                Terms. A waiver of any provision shall not constitute a waiver
                of any subsequent breach or default.
              </p>
              <p className="leading-[1.6] mb-0">​</p>
              <p className="leading-[1.6] mb-0">
                If any provision of these Terms is found to be invalid, illegal,
                or unenforceable, the remainder of these Terms shall remain in
                full force and effect. The invalid or unenforceable provision
                shall be modified or replaced to the extent necessary to make it
                valid and enforceable while maintaining the intent of the
                parties to the fullest extent permitted by law.
              </p>
              <p className="leading-[1.6] mb-0">​</p>
              <p className="leading-[1.6] mb-0">
                These Terms constitute the entire agreement between you and the
                Company regarding the subject matter herein and supersede all
                prior agreements, understandings, and representations, whether
                written or oral. No modifications or amendments to these Terms
                shall be binding unless made in writing and agreed upon by both
                parties.
              </p>
              <p className="leading-[1.6] mb-0">​</p>
              <p className="leading-[1.6] mb-0">
                The Company may assign or transfer its rights and obligations
                under these Terms to any other entity, including through merger,
                acquisition, corporate restructuring, or novation. By continuing
                to use the Service, you consent to any such transfer or
                assignment, and a notice posted on the Service indicating the
                change shall constitute valid notification.
              </p>
              <p className="leading-[1.6] mb-0">​</p>
              <p className="leading-[1.6] mb-0">
                All communications between you and the Company, including
                notices, disclosures, and agreements, shall be conducted
                electronically. You acknowledge that electronic communications,
                including emails, platform notifications, and digital
                agreements, hold the same legal weight as written documents and
                constitute a legally binding contract. By clicking buttons
                labelled “SUBMIT,” “CONTINUE,” “REGISTER,” or “I AGREE”, you
                affirm your intent to be legally bound by these Terms and
                acknowledge that your electronic submission constitutes a valid
                electronic signature.
              </p>
              <p className="leading-[1.6] mb-0">​</p>
              <p className="leading-[1.6] mb-0">
                The Company utilizes third-party providers to facilitate various
                operational and technical functions, including but not limited
                to payment processing, customer support, security enhancements,
                and data management. By using the Service, you acknowledge and
                agree that these third-party service providers may assist in
                delivering the Service and enhancing its functionality.
              </p>
              <p className="leading-[1.6] mb-0">​</p>
              <p className="leading-[1.6] mb-0">
                The Company shall not be liable for any failure or delay in
                complying with these Terms where such failure arises from
                circumstances beyond its reasonable control, including but not
                limited to force majeure events, legal or regulatory changes,
                cyberattacks, or unforeseen operational disruptions.
              </p>
              <p className="leading-[1.6] mb-0">​</p>
              <p className="leading-[1.6] mb-0">
                By continuing to use the Service, you acknowledge that you have
                read, understood, and agreed to these Terms in their entirety.
              </p>
              <p className="leading-[1.6] mb-0">​</p>
              <p className="leading-[1.6] mb-0">
                If you access or download the Services through App Store or
                Google Play (“App Store(s)”), you acknowledge that you have
                read, understood, and agree to the following:
              </p>
              <p className="leading-[1.6] mb-0">​</p>
              <p className="leading-[1.6] mb-0">
                • These Terms are between you and the Company only, and not with
                the App Stores. The Company, and not the App Stores, is solely
                responsible for the Services and their content, including any
                maintenance, support, warranties, product liability, failure to
                conform to applicable legal or regulatory requirements, consumer
                protection claims, and the investigation, defense, settlement,
                and resolution of any third-party intellectual property claims
                arising from the Services or your use thereof. The App Stores
                are not liable for your compliance or non-compliance with these
                Terms.
              </p>
              <p className="leading-[1.6]">
                • The license granted to you for the Services is personal,
                limited, non-exclusive, and non-transferable, permitting you to
                install and use the Services only on devices you own or control,
                strictly for personal, non-commercial purposes and subject to
                the applicable App Store’s terms of service. If the Services
                fail to conform to any applicable warranty, you may notify the
                applicable App Store, which may refund the purchase price, if
                any, paid for the Services in accordance with its policies. In
                the event of any conflict between these Terms and the applicable
                terms or policies of the App Store through which you downloaded
                the Services, the applicable App Store’s terms shall govern
                solely with respect to such conflict. Your use of the Services
                must comply with all applicable third-party agreements,
                including the applicable App Store’s terms of service.
              </p>
            </div>
          </section>
          <div
            className="h-0 relative shrink-0 w-full"
            data-node-id="813:2418"
            data-name="Divider / Horizontal / 03"
          >
            <div className="absolute inset-[-1px_0_0_0]">
              <img
                alt=""
                className="block max-w-none size-full"
                src={imgDividerHorizontal1}
              />
            </div>
          </div>
          <section
            className="[word-break:break-word] content-stretch flex flex-col gap-[8px] items-start relative shrink-0 w-full figma-section"
            data-node-id="813:2419"
            data-name="Section / Contact Info"
          >
            <p
              className="font-display leading-[normal] not-italic relative shrink-0 text-[#1a1612] text-[14px] w-full"
              data-node-id="813:2420"
            >
              16.1 CONTACT INFORMATION
            </p>
            <p
              className="font-serif font-normal leading-[1.6] relative shrink-0 text-[#2d241e] text-[15px] w-full"
              data-node-id="813:2421"
            >
              For general inquiries or support, you may contact us at our
              Support center.
            </p>
            <p
              className="font-serif font-normal italic leading-[normal] relative shrink-0 text-[#555] text-[14px] w-full"
              data-node-id="813:2422"
            >
              Last updated: April 2, 2026
            </p>
          </section>
          <div
            className="content-stretch flex items-start justify-center pt-[24px] relative shrink-0 w-full"
            data-node-id="813:2423"
            data-name="Container / Decorative Divider"
          >
            <div
              className="relative shrink-0 size-[24px]"
              data-node-id="813:2424"
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

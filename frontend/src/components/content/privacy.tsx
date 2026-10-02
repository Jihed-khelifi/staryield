/* Figma 822:2319 — reusable responsive content regions. */
/* eslint-disable @next/next/no-img-element */
import { ContentLink } from "@/components/content/content-link";
const assetPathPrefix = "https://assets.staryield.net/assets/figma";
const imgMediaPrivacyPolicy = `${assetPathPrefix}/fb9ee.jpg`;
const imgDividerHorizontal = `${assetPathPrefix}/27235.svg`;
export default function ScreenDesktopLegalPrivacyPolicy() {
  return (
    <div
      className="figma-content flex w-full flex-col items-center bg-cream"
      data-node-id="822:2319"
      data-name="Screen / Desktop / Legal / Privacy Policy"
    >
      <section
        className="content-stretch flex flex-col gap-[16px] items-center pb-[40px] pt-[80px] px-6 md:px-10 lg:px-20 relative shrink-0 w-full figma-section"
        data-node-id="822:2350"
        data-name="Section / Hero"
      >
        <div
          className="content-stretch flex items-center relative shrink-0"
          data-node-id="822:2351"
          data-name="Badge / STARYIELD PRIVACY AGREEMENT"
        >
          <p
            className="[word-break:break-word] font-serif font-semibold leading-[normal] relative shrink-0 text-[#c29a3d] text-[14px] tracking-[3px] uppercase whitespace-nowrap"
            data-node-id="822:2352"
          >
            STARYIELD PRIVACY AGREEMENT
          </p>
        </div>
        <h1
          className="[word-break:break-word] font-display leading-[1.2] not-italic relative shrink-0 text-[#1a1612] text-[48px] text-center whitespace-nowrap"
          data-node-id="822:2353"
        >
          Privacy Policy
        </h1>
        <div
          className="h-0 relative shrink-0 w-[120px]"
          data-node-id="822:2354"
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
        data-node-id="822:2355"
        data-name="Container / Legal Content Container"
      >
        <div
          className="border border-[#aa9063] border-solid content-stretch drop-shadow-[0px_12px_16px_rgba(0,0,0,0.04)] flex flex-col gap-[28px] items-start px-[64px] py-[72px] relative rounded-[12px] shrink-0 w-[974px]"
          data-node-id="822:2356"
          data-name="Card / Legal Sheet"
        >
          <div
            aria-hidden
            className="absolute inset-0 pointer-events-none rounded-[12px]"
          >
            <div className="absolute bg-[var(--cream)] inset-0 rounded-[12px]" />
            <div
              className="absolute bg-size-[1920px_1603px] bg-top-left inset-0 mix-blend-overlay rounded-[12px]"
              style={{ backgroundImage: `url("${imgMediaPrivacyPolicy}")` }}
            />
          </div>
          <div
            className="[word-break:break-word] font-serif font-normal leading-[0] relative shrink-0 text-[#2d241e] text-[14px] w-full whitespace-pre-wrap"
            data-node-id="826:2474"
          >
            <p className="leading-[1.6] mb-[10px]">IMPORTANT INFORMATION</p>
            <p className="leading-[1.6] mb-[10px]">​</p>
            <p className="leading-[1.6] mb-[10px]">
              To use the Services, we may ask you to enter your gender, date of
              birth, place of birth, and time of birth. We also automatically
              collect language settings, IP address, time zone, type and model
              of your device, device settings, operating system, Internet
              Service provider, mobile carrier, hardware ID, Facebook ID, and
              other unique identifiers (such as IDFA and GAID) from your device.
              We need this data to provide Services, analyze how our customers
              use Services, and serve advertising.
            </p>
            <p className="leading-[1.6] mb-[10px]">​</p>
            <p className="leading-[1.6] mb-[10px]">
              To improve Services and serve advertising, we may share this data
              with third parties, including Meta, TikTok, Google, Apple,
              Appsflyer, Amplitude, Firebase, Customer.io, and Twilio. We share
              this data with third parties to (1) analyze how users interact
              with Services (e.g., how often users make subscriptions,) and (2)
              serve and show advertising only to a particular group of users
              (e.g., to subscribers). Read this Policy to get more information
              about what data we collect (Section 2), how we use it (Section 3),
              with whom we share it (Section 4), and what data privacy rights
              are available to you (Section 5).
            </p>
            <p className="leading-[1.6] mb-[10px]">​</p>
            <p className="leading-[1.6] mb-[10px]">
              If you are a US resident, check our Privacy Notice for U.S.
              Residents.
            </p>
            <p className="leading-[1.6] mb-[10px]">​</p>
            <p className="leading-[1.6] mb-[10px]">PRIVACY POLICY</p>
            <p className="leading-[1.6] mb-[10px]">​</p>
            <p className="leading-[1.6] mb-[10px]">
              This Privacy Policy explains how we (“the Company”, “Provider”,
              “we”) collect, use, store, and share your personal data when you
              use our mobile applications, websites, and related services
              (collectively, the “App” or “Services”).
            </p>
            <p className="leading-[1.6] mb-[10px]">​</p>
            <p className="leading-[1.6] mb-[10px]">
              BY USING THE SERVICES, YOU PROMISE US THAT (I) YOU HAVE READ,
              UNDERSTAND, AND AGREE TO THIS PRIVACY POLICY, AND (II) YOU ARE AT
              LEAST 18 YEARS OLD (OR HAVE HAD YOUR PARENT OR GUARDIAN READ AND
              AGREE TO THIS PRIVACY POLICY FOR YOU). If you do not agree, or are
              unable to make this promise, you must not use the Services. In
              such case, you must (a) delete your account and personal data
              using the account settings in the App, (b) cancel any
              subscriptions you have using the account settings in the App or
              the functionality provided by Apple (for iOS device) or Google
              (for Android device), and (c) delete the App from your devices.
            </p>
            <p className="leading-[1.6] mb-[10px]">​</p>
            <p className="leading-[1.6]">{`Any translation from the English version is provided only for transparency and your convenience. If there is any difference in meaning or interpretation between the English language version of this Privacy Policy available at https://staryield.com/app/privacy and any translation, the English language version will prevail. The original English text shall be the sole legally binding version.`}</p>
          </div>
          <div
            className="[word-break:break-word] bg-[#f8f4ea] border border-[rgba(176,145,79,0.45)] border-solid content-stretch flex flex-col gap-[12px] items-start overflow-clip px-[32px] py-[28px] relative rounded-[8px] shrink-0 text-[#2d241e] w-full"
            data-node-id="826:2478"
            data-name="Container / Table Of Contents — Navigation Links"
          >
            <p
              className="font-display leading-[1.5] min-w-full not-italic relative shrink-0 text-[18px] w-[min-content]"
              data-node-id="826:2479"
            >
              TABLE OF CONTENTS
            </p>
            <ContentLink
              className="block cursor-pointer font-serif font-semibold leading-[0] min-w-full relative shrink-0 text-[15px] text-left w-[min-content]"
              data-node-id="826:2480"
              label="PERSONAL DATA CONTROLLER"
            >
              <ol className="list-decimal" start={1}>
                <li className="ms-[22.5px]">
                  <span className="leading-[1.5]">
                    PERSONAL DATA CONTROLLER
                  </span>
                </li>
              </ol>
            </ContentLink>
            <ContentLink
              className="block cursor-pointer font-serif font-semibold leading-[0] min-w-full relative shrink-0 text-[15px] text-left w-[min-content]"
              data-node-id="826:2481"
              label="CATEGORIES OF PERSONAL DATA WE COLLECT"
            >
              <ol className="list-decimal" start={2}>
                <li className="ms-[22.5px]">
                  <span className="leading-[1.5]">
                    CATEGORIES OF PERSONAL DATA WE COLLECT
                  </span>
                </li>
              </ol>
            </ContentLink>
            <ContentLink
              className="block cursor-pointer font-serif font-normal leading-[0] relative shrink-0 text-[14px] text-left w-[806px]"
              data-node-id="826:2482"
              label="2.1. Data you directly provide to us"
            >
              <p className="leading-[1.5] whitespace-pre-wrap">{`    2.1. Data you directly provide to us`}</p>
            </ContentLink>
            <ContentLink
              className="block cursor-pointer font-serif font-normal leading-[0] relative shrink-0 text-[14px] text-left w-[806px]"
              data-node-id="826:2483"
              label="2.2. Data we receive from third parties"
            >
              <p className="leading-[1.5] whitespace-pre-wrap">{`    2.2. Data we receive from third parties`}</p>
            </ContentLink>
            <ContentLink
              className="block cursor-pointer font-serif font-normal leading-[0] relative shrink-0 text-[14px] text-left w-[806px]"
              data-node-id="826:2484"
              label="2.3. Data we collect automatically"
            >
              <p className="leading-[1.5] whitespace-pre-wrap">{`    2.3. Data we collect automatically`}</p>
            </ContentLink>
            <ContentLink
              className="block cursor-pointer font-serif font-normal leading-[0] relative shrink-0 text-[14px] text-left w-[806px]"
              data-node-id="826:2485"
              label="2.4. Online store data"
            >
              <p className="leading-[1.5] whitespace-pre-wrap">{`    2.4. Online store data`}</p>
            </ContentLink>
            <ContentLink
              className="block cursor-pointer font-serif font-normal leading-[0] relative shrink-0 text-[14px] text-left w-[806px]"
              data-node-id="826:2486"
              label="2.5. Aggregated and de-Identified data"
            >
              <p className="leading-[1.5] whitespace-pre-wrap">{`    2.5. Aggregated and de-Identified data`}</p>
            </ContentLink>
            <ContentLink
              className="block cursor-pointer font-serif font-semibold leading-[0] min-w-full relative shrink-0 text-[15px] text-left w-[min-content]"
              data-node-id="826:2487"
              label="PURPOSES AND LEGAL BASES FOR PROCESSING YOUR PERSONAL DATA"
            >
              <ol className="list-decimal" start={3}>
                <li className="ms-[22.5px]">
                  <span className="leading-[1.5]">
                    PURPOSES AND LEGAL BASES FOR PROCESSING YOUR PERSONAL DATA
                  </span>
                </li>
              </ol>
            </ContentLink>
            <ContentLink
              className="block cursor-pointer font-serif font-semibold leading-[0] min-w-full relative shrink-0 text-[15px] text-left w-[min-content]"
              data-node-id="826:2488"
              label="HOW WE SHARE YOUR PERSONAL DATA"
            >
              <ol className="list-decimal" start={4}>
                <li className="ms-[22.5px]">
                  <span className="leading-[1.5]">
                    HOW WE SHARE YOUR PERSONAL DATA
                  </span>
                </li>
              </ol>
            </ContentLink>
            <ContentLink
              className="block cursor-pointer font-serif font-semibold leading-[0] min-w-full relative shrink-0 text-[15px] text-left w-[min-content]"
              data-node-id="826:2489"
              label="YOUR PRIVACY RIGHTS"
            >
              <ol className="list-decimal" start={5}>
                <li className="ms-[22.5px]">
                  <span className="leading-[1.5]">YOUR PRIVACY RIGHTS</span>
                </li>
              </ol>
            </ContentLink>
            <ContentLink
              className="block cursor-pointer font-serif font-semibold leading-[0] min-w-full relative shrink-0 text-[15px] text-left w-[min-content]"
              data-node-id="826:2490"
              label="AGE LIMITATION"
            >
              <ol className="list-decimal" start={6}>
                <li className="ms-[22.5px]">
                  <span className="leading-[1.5]">AGE LIMITATION</span>
                </li>
              </ol>
            </ContentLink>
            <ContentLink
              className="block cursor-pointer font-serif font-semibold leading-[0] min-w-full relative shrink-0 text-[15px] text-left w-[min-content]"
              data-node-id="826:2491"
              label="INTERNATIONAL DATA TRANSFERS"
            >
              <ol className="list-decimal" start={7}>
                <li className="ms-[22.5px]">
                  <span className="leading-[1.5]">
                    INTERNATIONAL DATA TRANSFERS
                  </span>
                </li>
              </ol>
            </ContentLink>
            <ContentLink
              className="block cursor-pointer font-serif font-semibold leading-[0] min-w-full relative shrink-0 text-[15px] text-left w-[min-content]"
              data-node-id="826:2492"
              label="DATA RETENTION"
            >
              <ol className="list-decimal" start={8}>
                <li className="ms-[22.5px]">
                  <span className="leading-[1.5]">DATA RETENTION</span>
                </li>
              </ol>
            </ContentLink>
            <ContentLink
              className="block cursor-pointer font-serif font-semibold leading-[0] min-w-full relative shrink-0 text-[15px] text-left w-[min-content]"
              data-node-id="826:2493"
              label="PRIVACY NOTICE FOR U.S. RESIDENTS"
            >
              <ol className="list-decimal" start={9}>
                <li className="ms-[22.5px]">
                  <span className="leading-[1.5]">
                    PRIVACY NOTICE FOR U.S. RESIDENTS
                  </span>
                </li>
              </ol>
            </ContentLink>
            <ContentLink
              className="block cursor-pointer font-serif font-normal leading-[0] relative shrink-0 text-[14px] text-left w-[806px]"
              data-node-id="826:2494"
              label="9.1. Applicability"
            >
              <p className="leading-[1.5] whitespace-pre-wrap">{`    9.1. Applicability`}</p>
            </ContentLink>
            <ContentLink
              className="block cursor-pointer font-serif font-normal leading-[0] relative shrink-0 text-[14px] text-left w-[806px]"
              data-node-id="826:2495"
              label="9.2. Categories of Personal Information We Collect"
            >
              <p className="leading-[1.5] whitespace-pre-wrap">{`    9.2. Categories of Personal Information We Collect`}</p>
            </ContentLink>
            <ContentLink
              className="block cursor-pointer font-serif font-normal leading-[0] relative shrink-0 text-[14px] text-left w-[806px]"
              data-node-id="826:2496"
              label="9.3. Purposes of Processing Personal Information"
            >
              <p className="leading-[1.5] whitespace-pre-wrap">{`    9.3. Purposes of Processing Personal Information`}</p>
            </ContentLink>
            <ContentLink
              className="block cursor-pointer font-serif font-normal leading-[0] relative shrink-0 text-[14px] text-left w-[806px]"
              data-node-id="826:2497"
              label="9.4. Categories of Personal Information We Disclose and Share with Third Parties"
            >
              <p className="leading-[1.5] whitespace-pre-wrap">{`    9.4. Categories of Personal Information We Disclose and Share with Third Parties`}</p>
            </ContentLink>
            <ContentLink
              className="block cursor-pointer font-serif font-normal leading-[0] relative shrink-0 text-[14px] text-left w-[806px]"
              data-node-id="826:2498"
              label="9.5. Your Rights"
            >
              <p className="leading-[1.5] whitespace-pre-wrap">{`    9.5. Your Rights`}</p>
            </ContentLink>
            <ContentLink
              className="block cursor-pointer font-serif font-semibold leading-[0] min-w-full relative shrink-0 text-[15px] text-left w-[min-content]"
              data-node-id="826:2499"
              label="SECURITY MEASURES"
            >
              <ol className="list-decimal" start={10}>
                <li className="ms-[22.5px]">
                  <span className="leading-[1.5]">SECURITY MEASURES</span>
                </li>
              </ol>
            </ContentLink>
            <ContentLink
              className="block cursor-pointer font-serif font-semibold leading-[0] min-w-full relative shrink-0 text-[15px] text-left w-[min-content]"
              data-node-id="826:2500"
              label="THIRD-PARTY WEBSITES AND APPLICATIONS"
            >
              <ol className="list-decimal" start={11}>
                <li className="ms-[22.5px]">
                  <span className="leading-[1.5]">
                    THIRD-PARTY WEBSITES AND APPLICATIONS
                  </span>
                </li>
              </ol>
            </ContentLink>
            <ContentLink
              className="block cursor-pointer font-serif font-semibold leading-[0] min-w-full relative shrink-0 text-[15px] text-left w-[min-content]"
              data-node-id="826:2501"
              label="CHANGES TO THIS PRIVACY POLICY"
            >
              <ol className="list-decimal" start={12}>
                <li className="ms-[22.5px]">
                  <span className="leading-[1.5]">
                    CHANGES TO THIS PRIVACY POLICY
                  </span>
                </li>
              </ol>
            </ContentLink>
            <ContentLink
              className="block cursor-pointer font-serif font-semibold leading-[0] min-w-full relative shrink-0 text-[15px] text-left w-[min-content]"
              data-node-id="826:2502"
              label="CONTACT US"
            >
              <ol className="list-decimal" start={13}>
                <li className="ms-[22.5px]">
                  <span className="leading-[1.5]">CONTACT US</span>
                </li>
              </ol>
            </ContentLink>
          </div>
          <div
            className="content-stretch flex flex-col items-start overflow-clip relative shrink-0 w-full"
            data-node-id="826:2504"
            data-name="Container / Chapter 01 — PERSONAL DATA CONTROLLER"
          >
            <div
              className="[word-break:break-word] font-serif font-normal leading-[0] relative shrink-0 text-[#2d241e] text-[0px] w-full whitespace-pre-wrap"
              data-node-id="826:2505"
            >
              <h2
                className="font-serif font-bold leading-[1.6] mb-[10px] text-[18px]"
                id="personal-data-controller"
              >
                1. PERSONAL DATA CONTROLLER
              </h2>
              <p className="leading-[1.6] mb-[10px] text-[14px]">​</p>
              <p className="leading-[1.6] text-[14px]">
                Staryield Limited (with registered office at 62 Athalassas
                Avenue, Mezzanine Floor, Strovolos, 2012 Nicosia, Cyprus) is the
                controller of your personal data in most circumstances. In some
                situations, the Company acts only as a platform enabling you to
                interact with independent third parties (for example, when you
                use our services to connect with advisors (psychics)). In those
                cases, the relevant third party is the controller of your
                personal data, and the Company is a data processor.
              </p>
            </div>
          </div>
          <div
            className="content-stretch flex flex-col gap-[20px] items-start overflow-clip relative shrink-0 w-full"
            data-node-id="826:2506"
            data-name="Container / Chapter 02 — CATEGORIES OF PERSONAL DATA WE COLLECT"
          >
            <div
              className="[word-break:break-word] font-serif font-bold leading-[0] relative shrink-0 text-[#2d241e] text-[18px] w-full whitespace-pre-wrap"
              data-node-id="826:2507"
            >
              <h2
                className="leading-[1.6] mb-[10px]"
                id="categories-of-personal-data-we-collect"
              >
                2. CATEGORIES OF PERSONAL DATA WE COLLECT
              </h2>
              <p className="leading-[1.6] mb-[10px]">​</p>
              <p className="leading-[1.6] mb-[10px]">
                We collect the following data about you:
              </p>
              <p className="leading-[1.6] mb-[10px]">​</p>
              <p className="leading-[1.6] mb-[10px]">{`    Data you directly provide to us (for example, date of birth).`}</p>
              <p className="leading-[1.6] mb-[10px]">{`    Data we receive about you from third parties (for example, when you sign in via Google).`}</p>
              <p className="leading-[1.6]">{`    Data we collect automatically when you use the Services (for example, your IP address).`}</p>
            </div>
            <div
              className="content-stretch flex flex-col items-start overflow-clip pl-[24px] relative shrink-0 w-full"
              data-node-id="826:2530"
              data-name="Container / Subchapter 2.1 — Data You Directly Provide To Us"
            >
              <div
                className="[word-break:break-word] font-serif font-normal leading-[0] relative shrink-0 text-[#2d241e] text-[0px] w-full whitespace-pre-wrap"
                data-node-id="826:2531"
              >
                <p className="font-serif font-bold leading-[1.6] mb-[10px] text-[16px]">
                  2.1. Data you directly provide to us
                </p>
                <p className="leading-[1.6] mb-[10px] text-[14px]">​</p>
                <p className="leading-[1.6] mb-[10px] text-[14px]">
                  Identifiers. This may include your name, phone number, and
                  email address. You provide us with this information when you
                  register for the Services, purchase the Services, subscribe to
                  our newsletters or other communications, or contact us by any
                  other means.
                </p>
                <p className="leading-[1.6] mb-[10px] text-[14px]">​</p>
                <p className="leading-[1.6] mb-[10px] text-[14px]">
                  Onboarding and product data. You provide us with onboarding
                  information, quiz answers, and login details when you register
                  for the Services, go through the onboarding process, or use
                  the Services. This may include gender, date of birth, place of
                  birth, time of birth, login and password, and answers to quiz
                  questions.
                </p>
                <p className="leading-[1.6] mb-[10px] text-[14px]">​</p>
                <p className="leading-[1.6] mb-[10px] text-[14px]">
                  Palm or face image. As a part of the Onboarding data, we may
                  ask for your palm or face image to provide you with the
                  relevant parts of the Services, namely, to create a reading
                  for you. We process these images solely to create reading for
                  you. We do not process them for identification purposes. We
                  delete these images right after the reading is created. If you
                  do not purchase a reading after completing the onboarding
                  quiz, we may send you reminders to complete the purchase; in
                  such case, we store the image for a limited period up to 2
                  months to generate a reading if you decide to complete a
                  purchase after seeing a reminder.
                </p>
                <p className="leading-[1.6] mb-[10px] text-[14px]">​</p>
                <p className="leading-[1.6] mb-[10px] text-[14px]">
                  Communications data. When you interact with the advisors
                  (psychics) or our support team, we process the contents of
                  your conversations, including your inquiries, feedback,
                  images, or other information you include in such
                  communications. We also process the personal data and the
                  information you send us when you submit requests through any
                  contact, request, or feedback form available within the
                  Services or the App.
                </p>
                <p className="leading-[1.6] mb-[10px] text-[14px]">​</p>
                <p className="leading-[1.6] text-[14px]">
                  Payment data. When you make payments through the Services, you
                  provide financial account data, for example your credit card
                  number, to our third-party service providers acting as our
                  payment processors. We do not collect, store, or have access
                  to the full card details you provide to payment processors,
                  including your full credit card number, though we may receive
                  some limited information, such as credit card-related data
                  (including a secure token reflecting your payment method),
                  data about products or services purchased, date, time and
                  amount of the purchase, the type of payment method you used,
                  and limited digits of your card number.
                </p>
              </div>
            </div>
            <div
              className="content-stretch flex flex-col items-start overflow-clip pl-[24px] relative shrink-0 w-full"
              data-node-id="826:2532"
              data-name="Container / Subchapter 2.2 — Data We Receive From Third Parties"
            >
              <div
                className="[word-break:break-word] font-serif font-normal leading-[0] relative shrink-0 text-[#2d241e] text-[0px] w-full whitespace-pre-wrap"
                data-node-id="826:2533"
              >
                <p className="font-serif font-bold leading-[1.6] mb-[10px] text-[16px]">
                  2.2. Data we receive from third parties
                </p>
                <p className="leading-[1.6] mb-[10px] text-[14px]">​</p>
                <p className="leading-[1.6] mb-[10px] text-[14px]">
                  Third-party account ID numbers. When you create an account or
                  login in the App using your third-party account (Google,
                  Apple, Meta), we get personal data from that third party. This
                  data may include your account ID number, name, profile
                  picture, and verified email address.
                </p>
                <p className="leading-[1.6] mb-[10px] text-[14px]">​</p>
                <p className="leading-[1.6] mb-[10px] text-[14px]">
                  Apple Sign-in. When you sign in with Apple to register an
                  account in the App, we get personal data from your Apple ID
                  account. This data may include, in particular, your name and
                  verified email address. You may choose to share your real
                  email address or an anonymised email address that uses the
                  private email relay service. Apple will show you their
                  detailed privacy information on the sign in with the Apple
                  screen. Find more about sign-in with Apple here.
                </p>
                <p className="leading-[1.6] mb-[10px] text-[14px]">​</p>
                <p className="leading-[1.6] mb-[10px] text-[14px]">
                  Facebook Sign-In. When you log in using Facebook, we get
                  personal data from your Facebook account. This includes your
                  name and Facebook ID. Unless you opt out on the Facebook Login
                  screen, we will also collect your email address. For more
                  information, please refer to the Facebook Permissions
                  Reference (describes the categories of information, which
                  Facebook may share with third parties and the set of
                  requirements) and to the Facebook Data policy. In addition,
                  Facebook lets you control the choices you made when connecting
                  your Facebook profile to the Website on their Apps and
                  Websites page.
                </p>
                <p className="leading-[1.6] mb-[10px] text-[14px]">​</p>
                <p className="leading-[1.6] mb-[10px] text-[14px]">
                  Google Sign-In. When you log in using Google, we receive
                  personal data from your Google Account: name, email address,
                  and profile picture associated with your Google Account. You
                  can revoke access provided to us on the Apps Permissions page.
                  To know more about how Google processes your data, visit their
                  Privacy Policy.
                </p>
                <p className="leading-[1.6] mb-[10px] text-[14px]">​</p>
                <p className="leading-[1.6] text-[14px]">
                  Mobile application marketplace data. When you download the
                  Company’s mobile application from the marketplace, they
                  automatically share with us data regarding your interaction
                  with their platform and the marketplace’s webpage of our
                  application. This data may include your click time, time when
                  the application’s download process started, total time of
                  download, search keyword ID, and type of your conversion and
                  engagement with the marketplace.
                </p>
              </div>
            </div>
            <div
              className="content-stretch flex flex-col items-start overflow-clip pl-[24px] relative shrink-0 w-full"
              data-node-id="826:2534"
              data-name="Container / Subchapter 2.3 — Data We Collect Automatically"
            >
              <div
                className="[word-break:break-word] font-serif font-normal leading-[0] relative shrink-0 text-[#2d241e] text-[0px] w-full whitespace-pre-wrap"
                data-node-id="826:2535"
              >
                <p className="font-serif font-bold leading-[1.6] mb-[10px] text-[16px]">
                  2.3. Data we collect automatically
                </p>
                <p className="leading-[1.6] mb-[10px] text-[14px]">​</p>
                <p className="leading-[1.6] mb-[10px] text-[14px]">
                  Advertising performance data. We collect data about your
                  referring application or URL (the application or place on the
                  Web you were on when you tapped on our advertising). This may
                  include user ID number, name and type of the advertising
                  campaign, name and ID number of the advertising, and
                  advertising click time.
                </p>
                <p className="leading-[1.6] mb-[10px] text-[14px]">​</p>
                <p className="leading-[1.6] mb-[10px] text-[14px]">
                  Device and Location data. We collect data from your mobile
                  device. This may include device ID number, language settings,
                  user ID number, IP address, time zone, type and model of a
                  device, device settings, operating system, screen resolution,
                  Internet Service provider, mobile carrier, hardware ID, and
                  Facebook ID. Also, if your device settings allow us to do so,
                  we may collect your location data when you visit our websites
                  or open the App, including such parameters as city, region,
                  country, postal code, and designated market area.
                </p>
                <p className="leading-[1.6] mb-[10px] text-[14px]">​</p>
                <p className="leading-[1.6] mb-[10px] text-[14px]">
                  Online activity data and Consumption information. We record
                  how you interact with our Services. For example, we log what
                  features and content you interact with, how often you use the
                  Services or the App, how long you stay in the Services or the
                  App, what sections you use, your orders, whether you view
                  communications we send you, to what extent you consume in-app
                  purchases, and other. This category of personal data includes
                  Consumption Information, which may be shared with Apple’s App
                  Store.
                </p>
                <p className="leading-[1.6] mb-[10px] text-[14px]">​</p>
                <p className="leading-[1.6] mb-[10px] text-[14px]">
                  Advertising IDs. Depending on the operating system of your
                  device, we collect your Apple Identifier for Advertising
                  (“IDFA”), Apple Identifier for Vendors (“IDFV”), or Google
                  Advertising Identifier (“GAID”). You can typically reset these
                  numbers through the settings of your device’s operating
                  system; we do not control these settings or the reset process.
                </p>
                <p className="leading-[1.6] mb-[10px] text-[14px]">​</p>
                <p className="leading-[1.6] text-[14px]">
                  Cookies and similar tracking technologies. We employ
                  technologies (cookies, SDKs, etc.) to process your data to
                  enhance your user experience, optimize ads, and analyze
                  traffic. For example, we use proprietary cookies necessary to
                  operate the Services: they enable page loading, session
                  management, security and fraud prevention, remember your
                  preferences, and support evaluation of user interactions with
                  new features. They collect limited information about your use
                  of the Services, such as page requests, load times, error
                  logs, etc., for operational and diagnostic purposes. We also
                  use Meta Pixel, TikTok Pixel, and Google tracking technologies
                  to collect data about your actions while using the Services,
                  including what pages you visit, the time you spend on each
                  page, and the actions you take. This data is used to measure
                  the effectiveness of our advertising campaigns and to
                  personalize the content and advertising we show to customers.
                  Unless the applicable laws permit otherwise, we activate these
                  technologies upon your consent when you interact with the
                  Services, visit our websites, use the App, or enable certain
                  features of the Services. Disabling these technologies may
                  affect the functionality of certain features, although the
                  Services will remain usable. Furthermore, we and our partners
                  utilize targeting technologies to tailor advertising,
                  potentially displaying them to you at relevant moments. When
                  enabled, your interactions with the Services or the App could
                  result in seeing our advertising on social media platforms,
                  assisting us in measuring advertising campaign effectiveness.
                  Check our Cookie Policy for more information.
                </p>
              </div>
            </div>
            <div
              className="content-stretch flex flex-col items-start overflow-clip pl-[24px] relative shrink-0 w-full"
              data-node-id="826:2536"
              data-name="Container / Subchapter 2.4 — Online Store Data"
            >
              <div
                className="[word-break:break-word] font-serif font-normal leading-[0] relative shrink-0 text-[#2d241e] text-[0px] w-full whitespace-pre-wrap"
                data-node-id="826:2537"
              >
                <p className="font-serif font-bold leading-[1.6] mb-[10px] text-[16px]">
                  2.4. Online store data
                </p>
                <p className="leading-[1.6] mb-[10px] text-[14px]">​</p>
                <p className="leading-[1.6] mb-[10px] text-[14px]">
                  When you buy physical products in our online store, we collect
                  personal data you provide to us when you place an order. This
                  may include your name, email address, billing and shipping
                  addresses, phone number, and details about your order,
                  including discounts and delivery method. All order payments
                  are processed by Shopify Payments or your selected payment
                  provider; we do not receive full card details, only limited
                  information such as card type, last four digits, payment
                  token, and payment status.
                </p>
                <p className="leading-[1.6] mb-[10px] text-[14px]">​</p>
                <p className="leading-[1.6] mb-[10px] text-[14px]">
                  When you browse the online store, our service provider,
                  Shopify, collects and sends us your technical and usage
                  information. This information may include IP address, device
                  and browser type, cookie or session identifiers, pages viewed,
                  timestamps, referrer, and approximate location.
                </p>
                <p className="leading-[1.6] mb-[10px] text-[14px]">​</p>
                <p className="leading-[1.6] mb-[10px] text-[14px]">
                  If you choose to sign in with Shopify’s “Shop” login, Shopify
                  provides us with your Shop account identifier and verified
                  email address. If your account settings permit that, Shopify
                  may also provide us with your name and saved address and phone
                  number for faster checkout and account setup. You can manage
                  your preferences and access settings in the Shop app or at
                  Shopify website.
                </p>
                <p className="leading-[1.6] mb-[10px] text-[14px]">​</p>
                <p className="leading-[1.6] text-[14px]">
                  For more information on Shopify’s data processing practices,
                  please check Shopify’s Privacy Policy.
                </p>
              </div>
            </div>
            <div
              className="content-stretch flex flex-col items-start overflow-clip pl-[24px] relative shrink-0 w-full"
              data-node-id="826:2538"
              data-name="Container / Subchapter 2.5 — Aggregated And De Identified Data"
            >
              <div
                className="[word-break:break-word] font-serif font-normal leading-[0] relative shrink-0 text-[#2d241e] text-[0px] w-full whitespace-pre-wrap"
                data-node-id="826:2539"
              >
                <p className="font-serif font-bold leading-[1.6] mb-[10px] text-[16px]">
                  2.5. Aggregated and de-Identified data
                </p>
                <p className="leading-[1.6] mb-[10px] text-[14px]">​</p>
                <p className="leading-[1.6] text-[14px]">
                  We may aggregate, anonymize, or de-identify your personal data
                  to ensure it cannot reasonably be used to identify you
                  individually. This processed data may be used for statistical
                  analysis and trend identification, to improve our Services,
                  develop new features, or other lawful purposes. For example,
                  we may analyse aggregated online activity, payment, and
                  advertising performance data to identify common interest
                  patterns among customers, which helps us enhance our Services
                  and contribute to their overall quality. Such aggregated data
                  no longer constitutes personal information under applicable
                  privacy laws.
                </p>
              </div>
            </div>
          </div>
          <div
            className="content-stretch flex flex-col items-start overflow-clip relative shrink-0 w-full"
            data-node-id="826:2508"
            data-name="Container / Chapter 03 — PURPOSES AND LEGAL BASES FOR PROCESSING YOUR PERSONAL DATA"
          >
            <div
              className="[word-break:break-word] font-serif font-normal leading-[0] relative shrink-0 text-[#2d241e] text-[0px] w-full whitespace-pre-wrap"
              data-node-id="826:2509"
            >
              <h2
                className="font-serif font-bold leading-[1.6] mb-[10px] text-[18px]"
                id="purposes-and-legal-bases-for-processing-your-personal-data"
              >
                3. PURPOSES AND LEGAL BASES FOR PROCESSING YOUR PERSONAL DATA
              </h2>
              <p className="leading-[1.6] mb-[10px] text-[14px]">​</p>
              <p className="leading-[1.6] mb-[10px] text-[14px]">
                We collect and use your data primarily to (1) provide and
                improve our Services, (2) enhance overall quality of Services,
                and (3) attract new customers to our Services. Below is a
                detailed explanation of the legal bases for and purposes of
                processing your personal data.
              </p>
              <p className="leading-[1.6] mb-[10px] text-[14px]">​</p>
              <p className="leading-[1.6] mb-[10px] text-[14px]">
                Legal bases for and purposes of processing
              </p>
              <p className="leading-[1.6] mb-[10px] text-[14px]">​</p>
              <p className="leading-[1.6] mb-[10px] text-[14px]">
                Purpose of processing — Category of personal data — Lawful basis
              </p>
              <p className="leading-[1.6] mb-[10px] text-[14px]">​</p>
              <p className="leading-[1.6] mb-[10px] text-[14px]">
                To provide Services (e.g., provide content for you) and
                administer your account — All categories, including the
                Onboarding and product data and the palm/face image —
                Performance of a contract with you.
              </p>
              <p className="leading-[1.6] mb-[10px] text-[14px]">​</p>
              <p className="leading-[1.6] mb-[10px] text-[14px]">
                To process your orders — Online store data — Performance of a
                contract with you.
              </p>
              <p className="leading-[1.6] mb-[10px] text-[14px]">​</p>
              <p className="leading-[1.6] mb-[10px] text-[14px]">
                To research, analyze, and enhance performance of the Services —
                Identifiers; Onboarding and product data; Payment data; Mobile
                application marketplace data; Advertising performance data;
                Device and Location data; Online activity data; Online store
                data; Advertising IDs; Cookies and similar tracking technologies
                — Legitimate interest (check the description).
              </p>
              <p className="leading-[1.6] mb-[10px] text-[14px]">​</p>
              <p className="leading-[1.6] mb-[10px] text-[14px]">{`To ensure quality control and safety of the Services. This includes compliance with the standards specified in Trust & Safety Center; check more details here. — Identifiers; Communications data; Payments data; Device and Location data; Online activity data; Cookies and similar tracking technologies — Performance of contract (when we fulfil contractual terms); Your consent; Legitimate interest (check the description).`}</p>
              <p className="leading-[1.6] mb-[10px] text-[14px]">​</p>
              <p className="leading-[1.6] mb-[10px] text-[14px]">
                To provide customer support — Identifiers; Onboarding and
                product data; Communications data; Payment data; Advertising
                performance data; Device and Location data; Online activity
                data; Online store data; Cookies and similar tracking
                technologies — Your consent; Performance of a contract with you
                (when we fulfil contractual terms).
              </p>
              <p className="leading-[1.6] mb-[10px] text-[14px]">​</p>
              <p className="leading-[1.6] mb-[10px] text-[14px]">
                To enable communication with the advisors — Identifiers;
                Onboarding and product data; Communications data — Performance
                of a contract with you.
              </p>
              <p className="leading-[1.6] mb-[10px] text-[14px]">​</p>
              <p className="leading-[1.6] mb-[10px] text-[14px]">
                To process payments, refunds, and chargebacks — Identifiers;
                Payments data; Device and Location data; Online activity data;
                Online store data — Performance of a contract with you (when we
                fulfil contractual terms); Legal obligation (if we take action
                as required by the law); Your consent; Legitimate interest
                (check the description).
              </p>
              <p className="leading-[1.6] mb-[10px] text-[14px]">​</p>
              <p className="leading-[1.6] mb-[10px] text-[14px]">{`To enforce Terms of Use, investigate complaints from users and advisors (psychics), prevent and combat fraud, violations of law and misbehavior, and ensure informational security of the Services. This includes enforcement of the standards specified in Trust & Safety Center; check more details here — Identifiers; Onboarding and product data; Communications data; Payment data; Third-party account ID numbers; Device and Location data; Online activity data; Cookies and similar tracking technologies — Legal obligation (when we take action as required by the law); Legitimate interest (check the description).`}</p>
              <p className="leading-[1.6] mb-[10px] text-[14px]">​</p>
              <p className="leading-[1.6] mb-[10px] text-[14px]">
                To send communications about the Services. We may send them by
                emails, SMS, push or in-chat notifications. — Identifiers;
                Onboarding and product data; Communications data; Payment data;
                Mobile application marketplace data; Advertising performance
                data; Device and Location data; Online activity data; Online
                store data; Cookies and similar tracking technologies —
                Performance of a contract with you.
              </p>
              <p className="leading-[1.6] mb-[10px] text-[14px]">​</p>
              <p className="leading-[1.6] mb-[10px] text-[14px]">
                To send marketing communications. You may receive information
                about our special offers, new features, available Services, and
                reminders to complete a purchase. We may show you advertisements
                on Services and send you emails, push notifications, and SMS for
                marketing purposes. Note: each communication includes an
                unsubscribe option. — Identifiers; Onboarding and product data;
                Communications data; Payment data; Device and Location data;
                Online activity data; Online store data; Cookies and similar
                tracking technologies — Your consent; Legitimate interest (check
                the description).
              </p>
              <p className="leading-[1.6] mb-[10px] text-[14px]">​</p>
              <p className="leading-[1.6] mb-[10px] text-[14px]">
                To serve personalized advertising. For example, if you have
                installed our Services, you might see ads of our products in
                your Facebook feed. — Identifiers; Onboarding and product data;
                Payment data; Mobile application marketplace data; Advertising
                performance data; Device and Location data; Online activity
                data; Online store data; Advertising IDs; Cookies and tracking
                technologies — Your consent; Legitimate interest (check the
                description).
              </p>
              <p className="leading-[1.6] mb-[10px] text-[14px]">​</p>
              <p className="leading-[1.6] mb-[10px] text-[14px]">
                To analyze performance of advertising. This includes measuring
                impression accuracy, ensuring proper ad placement and quality,
                and confirming compliance with industry standards — Identifiers;
                Onboarding and product data; Payment data; Advertising
                performance data; Device and Location data; Online activity
                data; Online store data; Advertising IDs; Cookies and tracking
                technologies — Your consent; Legitimate interest (check the
                description).
              </p>
              <p className="leading-[1.6] mb-[10px] text-[14px]">​</p>
              <p className="leading-[1.6] mb-[10px] text-[14px]">
                To comply with legal obligations — All categories — Legal
                obligation.
              </p>
              <p className="leading-[1.6] mb-[10px] text-[14px]">​</p>
              <p className="leading-[1.6] mb-[10px] text-[14px]">
                Description of our legitimate interests
              </p>
              <p className="leading-[1.6] mb-[10px] text-[14px]">​</p>
              <p className="leading-[1.6] mb-[10px] text-[14px]">
                When researching, analyzing, and enhancing performance of the
                Services: to improve the Services by better understanding of
                users’ preferences, being able to provide a better user
                experience, making the use of the Services easier and more
                enjoyable, or introducing new features, offers and topics for
                discussion with our advisors (psychics).
              </p>
              <p className="leading-[1.6] mb-[10px] text-[14px]">​</p>
              <p className="leading-[1.6] mb-[10px] text-[14px]">{`When ensuring quality control and safety of the Services: to ensure communications between customers and advisors (psychics) safe, trusted, and comply with the laws, our agreements and policies, including rules from Trust & Safety Center; to resolve issues and disputes with customers; to respond to crash error system reports; to ensure that advisors (psychics) follow the professional integrity and interaction standards required on our platform.`}</p>
              <p className="leading-[1.6] mb-[10px] text-[14px]">​</p>
              <p className="leading-[1.6] mb-[10px] text-[14px]">
                When processing payments, refunds, and chargebacks: to protect
                our business from fraudulent claims; to make informed and
                efficient decisions regarding refund requests.
              </p>
              <p className="leading-[1.6] mb-[10px] text-[14px]">​</p>
              <p className="leading-[1.6] mb-[10px] text-[14px]">{`When enforcing Terms of Use, preventing and combating fraud violations of law, and misbehavior, and ensuring informational security of the Services: to enforce our legal rights, policies, and rules, including rules from Trust & Safety Center; to prevent and address fraud, violations of law, misbehaviour, and unauthorized use of the Services; to prevent attacks on and unauthorised access to the Services; to ensure compliance with our Terms of Use. This helps provide a secure and reliable Services for all users.`}</p>
              <p className="leading-[1.6] mb-[10px] text-[14px]">​</p>
              <p className="leading-[1.6] mb-[10px] text-[14px]">
                When sending marketing communications: in jurisdictions where
                consent is not required, our legitimate interest is to encourage
                you to use Services, for example, to notify you that the advisor
                (psychic) is online; to promote the Services, for example, if
                you didn’t purchased the Services, we may send you reminders
                emails inviting you to complete a purchase.
              </p>
              <p className="leading-[1.6] mb-[10px] text-[14px]">​</p>
              <p className="leading-[1.6] mb-[10px] text-[14px]">
                When serving personalized advertising: in jurisdictions where
                consent is not required, our legitimate interest is to promote
                the Services in a measured and reasonably targeted way, showing
                you content that’s more likely to be relevant to your interests
                rather than generic advertising.
              </p>
              <p className="leading-[1.6] mb-[10px] text-[14px]">​</p>
              <p className="leading-[1.6] mb-[10px] text-[14px]">
                When analyzing performance of advertising: in jurisdictions
                where consent is not required, our legitimate interest is to
                promote the Services in a measured and appropriate way and to
                ensure the accuracy and quality of our advertising campaigns and
                complying with industry standards. This helps us measure
                effectiveness and maintain transparent business practices.
              </p>
              <p className="leading-[1.6] mb-[10px] text-[14px]">​</p>
              <p className="leading-[1.6] mb-[10px] text-[14px]">
                Important information
              </p>
              <p className="leading-[1.6] mb-[10px] text-[14px]">​</p>
              <p className="leading-[1.6] text-[14px]">
                We do not process your personal data to make solely
                automated-based decisions which produce legal effects concerning
                you or affect you in a similarly significant way (profiling).
              </p>
            </div>
          </div>
          <div
            className="content-stretch flex flex-col items-start overflow-clip relative shrink-0 w-full"
            data-node-id="826:2510"
            data-name="Container / Chapter 04 — HOW WE SHARE YOUR PERSONAL DATA"
          >
            <div
              className="[word-break:break-word] font-serif font-normal leading-[0] relative shrink-0 text-[#2d241e] text-[0px] w-full whitespace-pre-wrap"
              data-node-id="826:2511"
            >
              <h2
                className="font-serif font-bold leading-[1.6] mb-[10px] text-[18px]"
                id="how-we-share-your-personal-data"
              >
                4. HOW WE SHARE YOUR PERSONAL DATA
              </h2>
              <p className="leading-[1.6] mb-[10px] text-[14px]">​</p>
              <p className="leading-[1.6] mb-[10px] text-[14px]">
                We share information with third parties that help us operate,
                provide, improve, integrate, customize, support, and market our
                Services. We may share some sets of personal data, in
                particular, for purposes and with parties indicated in Section 3
                of this Policy. We strive to conclude specific data processing
                agreements with all such parties to establish the rules for
                processing of your data specifically on our behalf and limited
                to it.
              </p>
              <p className="leading-[1.6] mb-[10px] text-[14px]">​</p>
              <p className="leading-[1.6] mb-[10px] text-[14px]">
                The types of third parties we share information with include the
                following:
              </p>
              <p className="leading-[1.6] mb-[10px] text-[14px]">​</p>
              <p className="leading-[1.6] mb-[10px] text-[14px]">
                Third-party providers. We share personal data with third parties
                that we engage to provide services or perform business functions
                on our behalf, based on our instructions. We process your
                personal information using the following types of service
                providers (and in certain cases, their subprocessors):
              </p>
              <p className="leading-[1.6] mb-[10px] text-[14px]">​</p>
              <p className="leading-[1.6] mb-[10px] text-[14px]">{`    Cloud storage: We use Amazon Web Services to host personal data and enable our Services to operate.`}</p>
              <p className="leading-[1.6] mb-[10px] text-[14px]">{`    Monitoring of the Services: We use Firebase Performance Monitoring and Firebase Crash Reporting to monitor infrastructure and performance of the Services, identify and resolve errors and crashes in our Services across different platforms.`}</p>
              <p className="leading-[1.6] mb-[10px] text-[14px]">{`    Data analytics:`}</p>
              <p className="leading-[1.6] mb-[10px] text-[14px]">{`        We use Appsflyer to attribute your data, research and analyse how users interact with the App, and measure performance of our advertising.`}</p>
              <p className="leading-[1.6] mb-[10px] text-[14px]">{`        We use Amplitude to record how you use and interact with the Services; this might include session recordings.`}</p>
              <p className="leading-[1.6] mb-[10px] text-[14px]">{`        We use GrowthBook to run A/B tests, evaluate feature performance, and improve the Services by analysing feature experiment results and user interactions with the Services.`}</p>
              <p className="leading-[1.6] mb-[10px] text-[14px]">{`        We use Meta Pixel, TikTok Pixel, and Google Analytics to analyze how you use the Services and measure the effectiveness of some advertising.`}</p>
              <p className="leading-[1.6] mb-[10px] text-[14px]">{`        We use Tableau from Salesforce to create interactive dashboards, charts, reports, and visualise data about your use and interaction with the Services.`}</p>
              <p className="leading-[1.6] mb-[10px] text-[14px]">{`        We use Microsoft Clarity to understand how you interact with the Services through session recordings and heatmaps.`}</p>
              <p className="leading-[1.6] mb-[10px] text-[14px]">{`    Gateway service and payment processing: We use various payment processing and payment gateway providers that help us connect with different banks and payment systems around the world. Due to the large number of payment processing providers we work with, we cannot list them all here. However, some examples are Solidgate, Pay.com, Checkout, and Worldpay.`}</p>
              <p className="leading-[1.6] mb-[10px] text-[14px]">{`    Email verification and login: We use Debounce.io to validate, check and verify email addresses of users. We also use Google, Meta, and Apple APIs to enable social sign-in features.`}</p>
              <p className="leading-[1.6] mb-[10px] text-[14px]">{`    Functionality of the Services:`}</p>
              <p className="leading-[1.6] mb-[10px] text-[14px]">{`        We use AstrologyAPI to generate readings and reports for you.`}</p>
              <p className="leading-[1.6] mb-[10px] text-[14px]">{`        We use OneTrust to display our cookie banner, manage your consent and privacy preferences, and store proof of consent. This is necessary to help us comply with the applicable laws.`}</p>
              <p className="leading-[1.6] mb-[10px] text-[14px]">{`    Communications:`}</p>
              <p className="leading-[1.6] mb-[10px] text-[14px]">{`        We use Zendesk and Intercom as a customer service software to enable the work of our customer support team and maintain the request, contact, and feedback forms. We also use Twilio to enable phone calls.`}</p>
              <p className="leading-[1.6] mb-[10px] text-[14px]">{`        We use Customer.io, Twilio, and WhatsApp to send service-related and marketing communications to you.`}</p>
              <p className="leading-[1.6] mb-[10px] text-[14px]">{`        We use Firebase and Apple Push Notification to send you service-related and transactional messages and notifications.`}</p>
              <p className="leading-[1.6] mb-[10px] text-[14px]">{`    Security: We use Cloudflare as a content delivery network provider to improve the security and performance of our websites and protect them from malicious attacks.`}</p>
              <p className="leading-[1.6] mb-[10px] text-[14px]">{`    Technical, administrative, and information service providers: From time to time, we may use various third-party tools to ensure project and task management, launch feedback forms and surveys, enable communication, collaboration and productivity across our departments, or automate workflow. In some cases, limited personal data may be shared with these service providers. Examples of such tools include Google Workspace, Slack, Notion, and Atlassian Jira.`}</p>
              <p className="leading-[1.6] mb-[10px] text-[14px]">{`    AI tools. We may use AI tools to automate and enhance certain customer‑facing processes, for example, powering chatbots and tailoring Services messages and notifications. These uses evolve over time, so the specific processes may change. For example, we use OpenAI to help tailor push notifications based on analysis of a de-identified, limited, recent portion of Communications data. For all such processing, we rely on private, tenant‑isolated versions of AI models from vetted providers. We do not use public AI models. We configure these tools so your personal data is not used by the providers to train their AI models. We send only de‑identified or minimized data to AI models.`}</p>
              <p className="leading-[1.6] mb-[10px] text-[14px]">{`    E-commerce. We use Shopify to maintain and manage our online store. In particular, Shopify allows us to display our products, track orders and inventory, and process payments. Some of your information such as your name, email address, shipping address, and payment details are transferred to Shopify when you make a purchase in our online store. It is necessary to let us fulfill your order and provide you with a smooth shopping experience.`}</p>
              <p className="leading-[1.6] mb-[10px] text-[14px]">{`    Advisors, consultants, and independent contractors. From time to time, we may share limited personal information with our independent contractors, consultants, third-party specialists, partners, and advisors. They process this information on our behalf to operate, maintain, secure, and improve the Services, and ensure legal and regulatory compliance. We require them to use the information only for these purposes, protect it appropriately, and keep it confidential.`}</p>
              <p className="leading-[1.6] mb-[10px] text-[14px]">{`    Marketing and advertising:`}</p>
              <p className="leading-[1.6] mb-[10px] text-[14px]">{`        We use Meta Ads Manager and Meta Custom Audience, TikTok Ads Manager, and Pinterest Ads Manager to choose audiences that will see our advertising and deliver advertising on Meta’s, TikTok’s, and Pinterest’s platforms.`}</p>
              <p className="leading-[1.6] mb-[10px] text-[14px]">{`        We use Google Ads and Bing Ads to deliver advertising to users in Google and Bing browsers.`}</p>
              <p className="leading-[1.6] mb-[10px] text-[14px]">{`        From time to time, we may use third-party ad networks, ad agencies, and direct publishers that provide us with advertising solutions for displaying our advertising on various websites and apps.`}</p>
              <p className="leading-[1.6] mb-[10px] text-[14px]">​</p>
              <p className="leading-[1.6] mb-[10px] text-[14px]">
                Law enforcement agencies and other public authorities. We may
                use and disclose personal data to enforce our Terms of Use, to
                protect our rights, privacy, safety, or property, and/or that of
                our affiliates, you or others, and to respond to requests from
                courts, law enforcement agencies, regulatory agencies, and other
                public and government authorities, or in other cases provided
                for by law.
              </p>
              <p className="leading-[1.6] mb-[10px] text-[14px]">​</p>
              <p className="leading-[1.6] mb-[10px] text-[14px]">
                Third parties as part of a merger or acquisition. As we develop
                our business, we may buy or sell assets or business offerings.
                Customers’ information is generally one of the transferred
                business assets in these types of transactions. We may also
                transfer such information in the course of a corporate
                transaction, such as the sale of our business, a divestiture,
                merger, consolidation, or asset sale, or in the unlikely event
                of bankruptcy.
              </p>
              <p className="leading-[1.6] mb-[10px] text-[14px]">​</p>
              <p className="leading-[1.6] text-[14px]">
                Affiliates. We may share your personal information with our
                partner organizations that are part of our corporate group.
                These are the companies that are owned by, own, or are
                jointly-owned with us. These partner organizations will use the
                information in ways that align with this Policy, ensuring that
                your privacy is respected and protected across all our
                affiliated services.
              </p>
            </div>
          </div>
          <div
            className="content-stretch flex flex-col items-start overflow-clip relative shrink-0 w-full"
            data-node-id="826:2512"
            data-name="Container / Chapter 05 — YOUR PRIVACY RIGHTS"
          >
            <div
              className="[word-break:break-word] font-serif font-normal leading-[0] relative shrink-0 text-[#2d241e] text-[0px] w-full whitespace-pre-wrap"
              data-node-id="826:2513"
            >
              <h2
                className="font-serif font-bold leading-[1.6] mb-[10px] text-[18px]"
                id="your-privacy-rights"
              >
                5. YOUR PRIVACY RIGHTS
              </h2>
              <p className="leading-[1.6] mb-[10px] text-[14px]">​</p>
              <p className="leading-[1.6] mb-[10px] text-[14px]">
                We encourage you to contact us first regarding any issues
                related to your personal data processing. Feel free to contact
                us using this request form.
              </p>
              <p className="leading-[1.6] mb-[10px] text-[14px]">​</p>
              <p className="leading-[1.6] mb-[10px] text-[14px]">
                To be in control of your personal data, you have the rights
                specified below. Depending on where you reside, the applicable
                laws may not grant you some of these rights or they may be named
                differently. If you are a US resident, check our Privacy Notice
                for U.S. Residents.
              </p>
              <p className="leading-[1.6] mb-[10px] text-[14px]">​</p>
              <p className="leading-[1.6] mb-[10px] text-[14px]">
                Your Rights — How to Exercise Them
              </p>
              <p className="leading-[1.6] mb-[10px] text-[14px]">​</p>
              <p className="leading-[1.6] mb-[10px] text-[14px]">
                Right to Access / Know
              </p>
              <p className="leading-[1.6] mb-[10px] text-[14px]">​</p>
              <p className="leading-[1.6] mb-[10px] text-[14px]">
                An individual’s right to receive information about what personal
                data we process, how we use it, and with whom we share it.
              </p>
              <p className="leading-[1.6] mb-[10px] text-[14px]">​</p>
              <p className="leading-[1.6] mb-[10px] text-[14px]">
                We provide this information throughout this Privacy Policy,
                including what categories of personal data we process (Section
                2), the purposes of and the legal bases for the processing
                (Section 3), and with whom we share personal data (Section 4).
                We encourage you to check this Policy to find the information
                you are interested in.
              </p>
              <p className="leading-[1.6] mb-[10px] text-[14px]">​</p>
              <p className="leading-[1.6] mb-[10px] text-[14px]">
                Right to Delete / Erase
              </p>
              <p className="leading-[1.6] mb-[10px] text-[14px]">​</p>
              <p className="leading-[1.6] mb-[10px] text-[14px]">
                An individual’s right to request the deletion of their personal
                data, except when the data deletion is limited under the
                applicable laws.
              </p>
              <p className="leading-[1.6] mb-[10px] text-[14px]">​</p>
              <p className="leading-[1.6] mb-[10px] text-[14px]">
                You can delete your personal data using “Delete Account”
                functionality in your account settings. Otherwise, you can send
                a data deletion request via this request form.
              </p>
              <p className="leading-[1.6] mb-[10px] text-[14px]">​</p>
              <p className="leading-[1.6] mb-[10px] text-[14px]">
                When you request to erase your personal data, we will use
                reasonable efforts to honor your request. In some cases we may
                be legally required to keep some of the data for a certain time
                as necessary to comply with our legal obligations, resolve
                disputes, and enforce our agreements; in such event, we will
                fulfill your request after we have complied with our
                obligations.
              </p>
              <p className="leading-[1.6] mb-[10px] text-[14px]">​</p>
              <p className="leading-[1.6] mb-[10px] text-[14px]">
                Right to Rectify / Correct
              </p>
              <p className="leading-[1.6] mb-[10px] text-[14px]">​</p>
              <p className="leading-[1.6] mb-[10px] text-[14px]">
                An individual’s right to request the correction of inaccurate
                personal data about that individual.
              </p>
              <p className="leading-[1.6] mb-[10px] text-[14px]">​</p>
              <p className="leading-[1.6] mb-[10px] text-[14px]">
                You can easily correct (rectify) your personal data in the
                account settings of your account settings.
              </p>
              <p className="leading-[1.6] mb-[10px] text-[14px]">​</p>
              <p className="leading-[1.6] mb-[10px] text-[14px]">
                Some information may not be available for updating in the
                account settings. This information is collected automatically on
                the system level and cannot be manually rectified due to its
                nature. Instead, it updates in real-time every time you use the
                Services. This approach is aimed to ensure integrity and
                confidentiality of your personal data and reflects the best
                practices recommended by the applicable laws.
              </p>
              <p className="leading-[1.6] mb-[10px] text-[14px]">​</p>
              <p className="leading-[1.6] mb-[10px] text-[14px]">
                Right to Withdraw / Revoke Consent
              </p>
              <p className="leading-[1.6] mb-[10px] text-[14px]">​</p>
              <p className="leading-[1.6] mb-[10px] text-[14px]">
                When the processing of individual’s personal data is based on
                their consent, an individual’s right to withdraw consent to the
                processing of personal data they earlier provided.
              </p>
              <p className="leading-[1.6] mb-[10px] text-[14px]">​</p>
              <p className="leading-[1.6] mb-[10px] text-[14px]">
                Only a limited number of processing activities is based on your
                consent; see Section 3 for details.
              </p>
              <p className="leading-[1.6] mb-[10px] text-[14px]">​</p>
              <p className="leading-[1.6] mb-[10px] text-[14px]">
                To revoke consent to the processing of your personal data for
                sending marketing communications, use the “Unsubscribe” link
                available in the email or submit your request using this request
                form.
              </p>
              <p className="leading-[1.6] mb-[10px] text-[14px]">​</p>
              <p className="leading-[1.6] mb-[10px] text-[14px]">
                Right to Access a Copy / Port
              </p>
              <p className="leading-[1.6] mb-[10px] text-[14px]">​</p>
              <p className="leading-[1.6] mb-[10px] text-[14px]">
                An individual’s right to request data controller to send them a
                copy of their personal data.
              </p>
              <p className="leading-[1.6] mb-[10px] text-[14px]">​</p>
              <p className="leading-[1.6] mb-[10px] text-[14px]">
                You can submit a request to send you a copy of your personal
                data using this request form.
              </p>
              <p className="leading-[1.6] mb-[10px] text-[14px]">​</p>
              <p className="leading-[1.6] mb-[10px] text-[14px]">
                Right to Object or Restrict Processing
              </p>
              <p className="leading-[1.6] mb-[10px] text-[14px]">​</p>
              <p className="leading-[1.6] mb-[10px] text-[14px]">
                In a limited amount of situations, an individual has a right to
                restrict the processing of or to object to the processing of
                their personal data.
              </p>
              <p className="leading-[1.6] mb-[10px] text-[14px]">​</p>
              <p className="leading-[1.6] mb-[10px] text-[14px]">
                You can submit a request objecting to the processing of your
                personal data or restricting the processing of your personal
                data using this request form.
              </p>
              <p className="leading-[1.6] mb-[10px] text-[14px]">​</p>
              <p className="leading-[1.6] mb-[10px] text-[14px]">
                In certain countries, including in the European Economic Area
                and the United Kingdom, you have a right to lodge a complaint
                with the appropriate data protection authority if you have
                concerns about how we process your personal data. You can find
                information about your data protection regulator in the European
                Economic Area here, and in the United Kingdom here.
              </p>
              <p className="leading-[1.6] mb-[10px] text-[14px]">​</p>
              <p className="leading-[1.6] mb-[10px] text-[14px]">
                We may be required to verify your identity before being able to
                process your request. The verification process may include
                confirming your name, age, email address, date of subscription
                purchase, date of account creation, or other relevant Services
                usage data that reasonably identify you as the account owner. We
                may also request additional proof of identity if necessary, but
                we strive to minimize the information required. For certain
                requests, we may send a verification code or ask you to sign a
                declaration under penalty of perjury to authenticate your
                identity.
              </p>
              <p className="leading-[1.6] mb-[10px] text-[14px]">​</p>
              <p className="leading-[1.6] text-[14px]">
                If you designate an authorized agent to exercise your rights on
                your behalf, they will need to provide the necessary
                verification information to confirm their identity and
                authorization.
              </p>
            </div>
          </div>
          <div
            className="content-stretch flex flex-col items-start overflow-clip relative shrink-0 w-full"
            data-node-id="826:2514"
            data-name="Container / Chapter 06 — AGE LIMITATION"
          >
            <div
              className="[word-break:break-word] font-serif font-normal leading-[0] relative shrink-0 text-[#2d241e] text-[0px] w-full whitespace-pre-wrap"
              data-node-id="826:2515"
            >
              <h2
                className="font-serif font-bold leading-[1.6] mb-[10px] text-[18px]"
                id="age-limitation"
              >
                6. AGE LIMITATION
              </h2>
              <p className="leading-[1.6] mb-[10px] text-[14px]">​</p>
              <p className="leading-[1.6] text-[14px]">
                We do not knowingly allow anyone under 18 to use the Services,
                and we do not knowingly collect or process personal data from
                anyone under 18. If you learn that anyone younger than 18 has
                used the Services or provided us with personal data, please
                contact us using this request form.
              </p>
            </div>
          </div>
          <div
            className="content-stretch flex flex-col items-start overflow-clip relative shrink-0 w-full"
            data-node-id="826:2516"
            data-name="Container / Chapter 07 — INTERNATIONAL DATA TRANSFERS"
          >
            <div
              className="[word-break:break-word] font-serif font-normal leading-[0] relative shrink-0 text-[#2d241e] text-[0px] w-full whitespace-pre-wrap"
              data-node-id="826:2517"
            >
              <h2
                className="font-serif font-bold leading-[1.6] mb-[10px] text-[18px]"
                id="international-data-transfers"
              >
                7. INTERNATIONAL DATA TRANSFERS
              </h2>
              <p className="leading-[1.6] mb-[10px] text-[14px]">​</p>
              <p className="leading-[1.6] mb-[10px] text-[14px]">
                We may transfer personal data to countries other than the
                country in which the data was originally collected (1) to
                provide the Services set forth in the Terms of Use and (2) to
                achieve purposes indicated in this Policy. If these countries do
                not have the same data protection laws as the country in which
                you initially provided the information, we employ special
                safeguards.
              </p>
              <p className="leading-[1.6] mb-[10px] text-[14px]">​</p>
              <p className="leading-[1.6] text-[14px]">
                In particular, if we transfer personal data originating from the
                European Economic Area or the United Kingdom area, to countries
                with adequate level of data protection, we rely on the adequacy
                decision of the European Commission for this country (details
                available here); for example, we transfer personal data to the
                USA based on the EU-U.S Data Privacy Framework program (check
                details here). If we transfer personal data originating from the
                European Economic Area or the United Kingdom area to countries
                without adequate level of data protection, we rely on the
                Standard Contractual Clauses approved by the European Commission
                (details available here).
              </p>
            </div>
          </div>
          <div
            className="content-stretch flex flex-col items-start overflow-clip relative shrink-0 w-full"
            data-node-id="826:2518"
            data-name="Container / Chapter 08 — DATA RETENTION"
          >
            <div
              className="[word-break:break-word] font-serif font-normal leading-[0] relative shrink-0 text-[#2d241e] text-[0px] w-full whitespace-pre-wrap"
              data-node-id="826:2519"
            >
              <h2
                className="font-serif font-bold leading-[1.6] mb-[10px] text-[18px]"
                id="data-retention"
              >
                8. DATA RETENTION
              </h2>
              <p className="leading-[1.6] mb-[10px] text-[14px]">​</p>
              <p className="leading-[1.6] mb-[10px] text-[14px]">
                We retain your personal data for as long as necessary to fulfill
                the purposes described in this Privacy Policy and the Terms of
                Use, including to provide Services to you. When determining the
                appropriate retention period, we consider the volume, nature,
                and sensitivity of the personal data; the data’s relevance to
                performing the contract and providing the Services (including
                the possibility of re-enrollment); the potential risk of harm
                from unauthorized use or disclosure; the purposes for which we
                process your personal data and whether those purposes can be
                achieved by other means; and applicable legal requirements,
                including mandatory retention periods, statutes of limitation,
                legal holds, and litigation requirements.
              </p>
              <p className="leading-[1.6] mb-[10px] text-[14px]">​</p>
              <p className="leading-[1.6] mb-[10px] text-[14px]">
                As a general rule, we keep your personal data for as long as you
                maintain a registered account or certain personal information
                with us. However, retention period may be shorter or longer
                depending on the lawful basis for processing (see Section 3 for
                more information):
              </p>
              <p className="leading-[1.6] mb-[10px] text-[14px]">​</p>
              <p className="leading-[1.6] mb-[10px] text-[14px]">{`    When we process data on a basis of consent: until you withdraw your consent.`}</p>
              <p className="leading-[1.6] mb-[10px] text-[14px]">{`    When we process data to perform a contract: for the duration of the contract.`}</p>
              <p className="leading-[1.6] mb-[10px] text-[14px]">{`    When we process data to pursue our legitimate interest: for the time needed to achieve the stated interest, considering necessity and impact on your privacy.`}</p>
              <p className="leading-[1.6] mb-[10px] text-[14px]">{`    When we process data to comply with legal obligation: until we fulfill the legal obligation imposed on us by the applicable laws.`}</p>
              <p className="leading-[1.6] mb-[10px] text-[14px]">​</p>
              <p className="leading-[1.6] text-[14px]">
                If multiple lawful bases apply, we keep the personal data until
                the latest applicable period expires. In any of the above
                situations, if you decide to delete your account, we delete or
                anonymise the related personal data unless retention is required
                to comply with our legal obligations, establish, exercise, or
                defend legal claims, resolve disputes, and enforce our
                agreements. For example, certain accounting and tax laws require
                us to store Payment Data for long periods of time to comply with
                laws we are subject to. Therefore, even if you submit a data
                deletion request, a small portion of the data that relates to
                our compliance obligations will be stored even after the request
                is satisfied.
              </p>
            </div>
          </div>
          <div
            className="content-stretch flex flex-col gap-[20px] items-start overflow-clip relative shrink-0 w-full"
            data-node-id="826:2520"
            data-name="Container / Chapter 09 — PRIVACY NOTICE FOR U.S. RESIDENTS"
          >
            <h2
              className="[word-break:break-word] font-serif font-bold leading-[1.6] relative shrink-0 text-[#2d241e] text-[18px] w-full"
              data-node-id="826:2521"
              id="privacy-notice-for-u-s-residents"
            >
              9. PRIVACY NOTICE FOR U.S. RESIDENTS
            </h2>
            <div
              className="content-stretch flex flex-col items-start overflow-clip pl-[24px] relative shrink-0 w-full"
              data-node-id="826:2540"
              data-name="Container / Subchapter 9.1 — Applicability"
            >
              <div
                className="[word-break:break-word] font-serif font-normal leading-[0] relative shrink-0 text-[#2d241e] text-[0px] w-full whitespace-pre-wrap"
                data-node-id="826:2541"
              >
                <p className="font-serif font-bold leading-[1.6] mb-[10px] text-[16px]">
                  9.1. Applicability
                </p>
                <p className="leading-[1.6] mb-[10px] text-[14px]">​</p>
                <p className="leading-[1.6] mb-[10px] text-[14px]">
                  This Privacy Notice for U.S. Residents (“Notice”) supplements
                  our Privacy Policy and provides disclosures required by laws
                  in the U.S. states that have enacted consumer privacy laws. If
                  you reside in one of these states, this section applies to
                  you. For California residents, this also serves as our
                  California Notice at Collection.
                </p>
                <p className="leading-[1.6] mb-[10px] text-[14px]">​</p>
                <p className="leading-[1.6] text-[14px]">
                  The scope of “Personal Information” may vary by state law.
                  Generally, it refers to information that identifies, relates
                  to, describes, is capable of being associated with, or could
                  reasonably be linked, directly or indirectly, with a
                  particular consumer or household.
                </p>
              </div>
            </div>
            <div
              className="content-stretch flex flex-col items-start overflow-clip pl-[24px] relative shrink-0 w-full"
              data-node-id="826:2542"
              data-name="Form / Subchapter 9.2 — Categories Of Personal Ination We Collect"
            >
              <div
                className="[word-break:break-word] font-serif font-normal leading-[0] relative shrink-0 text-[#2d241e] text-[0px] w-full whitespace-pre-wrap"
                data-node-id="826:2543"
              >
                <p className="font-serif font-bold leading-[1.6] mb-[10px] text-[16px]">
                  9.2. Categories of Personal Information We Collect
                </p>
                <p className="leading-[1.6] mb-[10px] text-[14px]">​</p>
                <p className="leading-[1.6] mb-[10px] text-[14px]">
                  Section 2 of our Privacy Policy specifies the categories of
                  Personal Information we collect and the categories of sources
                  from which we collect Personal Information.
                </p>
                <p className="leading-[1.6] mb-[10px] text-[14px]">​</p>
                <p className="leading-[1.6] mb-[10px] text-[14px]">
                  Some of the Personal Information we collect might be
                  considered as sensitive personal information under certain
                  U.S. states consumer privacy laws. We process this information
                  solely to perform the requested Services, ensure the security
                  of the App and Services, and maintain the quality and improve
                  the App and Services. This information includes the following:
                </p>
                <p className="leading-[1.6] mb-[10px] text-[14px]">​</p>
                <p className="leading-[1.6] mb-[10px] text-[14px]">{`    Contents of chat or phone call communications.`}</p>
                <p className="leading-[1.6] text-[14px]">{`    Username/account number in combination with a password or other credential to your account and a payment token for your payment method.`}</p>
              </div>
            </div>
            <div
              className="content-stretch flex flex-col items-start overflow-clip pl-[24px] relative shrink-0 w-full"
              data-node-id="826:2544"
              data-name="Form / Subchapter 9.3 — Purposes Of Processing Personal Ination"
            >
              <div
                className="[word-break:break-word] font-serif font-normal leading-[0] relative shrink-0 text-[#2d241e] text-[0px] w-full whitespace-pre-wrap"
                data-node-id="826:2545"
              >
                <p className="font-serif font-bold leading-[1.6] mb-[10px] text-[16px]">
                  9.3. Purposes of Processing Personal Information
                </p>
                <p className="leading-[1.6] mb-[10px] text-[14px]">​</p>
                <p className="leading-[1.6] text-[14px]">
                  For each category of Personal Information, Section 3 of our
                  Privacy Policy describes the intended purposes of collecting,
                  processing, and sharing Personal Information, including for
                  targeted advertising.
                </p>
              </div>
            </div>
            <div
              className="content-stretch flex flex-col items-start overflow-clip pl-[24px] relative shrink-0 w-full"
              data-node-id="826:2546"
              data-name="Form / Subchapter 9.4 — Categories Of Personal Ination We Disclose And Share With Third Parties"
            >
              <div
                className="[word-break:break-word] font-serif font-normal leading-[0] relative shrink-0 text-[#2d241e] text-[0px] w-full whitespace-pre-wrap"
                data-node-id="826:2547"
              >
                <p className="font-serif font-bold leading-[1.6] mb-[10px] text-[16px]">
                  9.4. Categories of Personal Information We Disclose and Share
                  with Third Parties
                </p>
                <p className="leading-[1.6] mb-[10px] text-[14px]">​</p>
                <p className="leading-[1.6] mb-[10px] text-[14px]">
                  For additional details regarding the third parties with whom
                  we share Personal Information and the purposes for that
                  sharing, please see Section 4 of this Privacy Policy.
                </p>
                <p className="leading-[1.6] mb-[10px] text-[14px]">​</p>
                <p className="leading-[1.6] mb-[10px] text-[14px]">
                  Please note: we do not disclose Personal Information to any
                  third parties for their direct marketing purposes. To our
                  reasonable knowledge, no third party to whom we disclose
                  Personal Information uses it for their own direct marketing
                  purposes.
                </p>
                <p className="leading-[1.6] mb-[10px] text-[14px]">​</p>
                <p className="leading-[1.6] mb-[10px] text-[14px]">
                  Please note: from time to time, we may engage technical,
                  administrative, and information service providers, advisors,
                  consultants, and independent contractors (check here) and may
                  disclose to them any categories of Personal Information
                  specified below for business purposes as necessary to operate
                  and support the Services. When we do so, we disclose only the
                  minimum information required for these purposes and apply
                  appropriate security and confidentiality measures.
                </p>
                <p className="leading-[1.6] mb-[10px] text-[14px]">​</p>
                <p className="leading-[1.6] mb-[10px] text-[14px]">
                  We disclose the following Personal Information for business
                  purposes, to ensure operation of our business, as specified
                  below:
                </p>
                <p className="leading-[1.6] mb-[10px] text-[14px]">​</p>
                <p className="leading-[1.6] mb-[10px] text-[14px]">
                  Enumerated Category of Personal Information under Cal. Civ.
                  Code Section 1798.140(v) — Third Parties to Whom We Disclose
                  Personal Information
                </p>
                <p className="leading-[1.6] mb-[10px] text-[14px]">​</p>
                <p className="leading-[1.6] mb-[10px] text-[14px]">
                  Identifiers (Account name, unique personal identifier, online
                  identifier, IP address, email address): Cloud Storage provider
                  (check here); Data analytics tools (check here); Payment
                  processing and gateway service providers (check here);
                  E-commerce platform service provider (check here); Customer
                  support and communications service providers (check here);
                  Service providers of the security tools (check here); Service
                  providers of marketing and advertising tools (check here);
                  Email verification and social media login API providers (check
                  here).
                </p>
                <p className="leading-[1.6] mb-[10px] text-[14px]">​</p>
                <p className="leading-[1.6] mb-[10px] text-[14px]">
                  Categories of Personal Information in Cal. Civ. Code Section
                  1798.80(e) (Name, address, telephone number, bank account
                  number, credit card number, debit card number, or any other
                  financial information) and Commercial information (Products or
                  services purchased, obtained, or considered, or other
                  purchasing or consuming histories or tendencies): Cloud
                  Storage provider (check here); Data analytics tools (check
                  here); Payment processing and gateway service providers (check
                  here); E-commerce platform service provider (check here);
                  Customer support and communications service providers (check
                  here); AI tools — only de‑identified or minimized data (check
                  here); Service providers of marketing and advertising tools
                  (check here).
                </p>
                <p className="leading-[1.6] mb-[10px] text-[14px]">​</p>
                <p className="leading-[1.6] mb-[10px] text-[14px]">
                  Characteristics of Protected Classifications under California
                  or Federal Law (Age (over 40), sex, including gender): Cloud
                  Storage provider (check here); Data analytics tools (check
                  here).
                </p>
                <p className="leading-[1.6] mb-[10px] text-[14px]">​</p>
                <p className="leading-[1.6] mb-[10px] text-[14px]">
                  Internet or other electronic network activity information
                  (Browsing history, search history, information regarding a
                  consumer’s interaction with an internet website application,
                  or advertisement): Cloud Storage provider (check here);
                  Service providers of services monitoring tools (check here);
                  Data analytics tools (check here); E-commerce platform service
                  provider (check here); AI tools — only de‑identified or
                  minimized data (check here); Service providers of marketing
                  and advertising tools (check here).
                </p>
                <p className="leading-[1.6] mb-[10px] text-[14px]">​</p>
                <p className="leading-[1.6] mb-[10px] text-[14px]">
                  Geolocation data (Approximate geolocation only): Cloud Storage
                  provider (check here); Data analytics tools (check here);
                  Service providers of the security tools (check here); Service
                  providers of marketing and advertising tools (check here).
                </p>
                <p className="leading-[1.6] mb-[10px] text-[14px]">​</p>
                <p className="leading-[1.6] mb-[10px] text-[14px]">
                  Sensitive personal information (Contents of communication,
                  account access information): Cloud Storage provider (check
                  here); Customer support and communications service providers
                  (check here).
                </p>
                <p className="leading-[1.6] mb-[10px] text-[14px]">​</p>
                <p className="leading-[1.6] mb-[10px] text-[14px]">
                  Inferences Drawn from Personal Information (Consumer profiles
                  reflecting a consumer’s preferences, characteristics,
                  psychological trends, predispositions, behavior, attitudes,
                  intelligence, abilities, and aptitudes): We do not collect or
                  process this information.
                </p>
                <p className="leading-[1.6] mb-[10px] text-[14px]">​</p>
                <p className="leading-[1.6] mb-[10px] text-[14px]">
                  Biometric information; Sensory Information (Audio, electronic,
                  visual, thermal, olfactory, or similar information);
                  Professional or employment-related information; Non-Public
                  Education Information (as defined in 20 U.S.C. 1232g; 34
                  C.F.R. Part 99) (Records that are directly related to a
                  student and maintained by an educational agency or
                  institution, or by a party acting for the agency or
                  institution): We do not collect or process this information.
                </p>
                <p className="leading-[1.6] mb-[10px] text-[14px]">​</p>
                <p className="leading-[1.6] mb-[10px] text-[14px]">
                  We do not sell or otherwise monetize Personal Information,
                  except for sharing it for cross-context behavioral advertising
                  as described below:
                </p>
                <p className="leading-[1.6] mb-[10px] text-[14px]">​</p>
                <p className="leading-[1.6] mb-[10px] text-[14px]">
                  Enumerated Category of Personal Information under Cal. Civ.
                  Code Section 1798.140(v) — Third Parties with Whom We Share
                  Personal Information
                </p>
                <p className="leading-[1.6] mb-[10px] text-[14px]">​</p>
                <p className="leading-[1.6] mb-[10px] text-[14px]">
                  Identifiers (Account name, unique personal identifier, online
                  identifier, IP address, email address); Commercial information
                  (Products or services purchased, obtained, or considered, or
                  other purchasing or consuming histories or tendencies);
                  Internet or other electronic network activity information
                  (Browsing history, search history, information regarding a
                  consumer’s interaction with an internet website application,
                  or advertisement); Geolocation data (Approximate geolocation
                  only): Service providers of marketing and advertising tools
                  (check here); Third-party ad networks, ad agencies, and direct
                  advertising publishers.
                </p>
                <p className="leading-[1.6] mb-[10px] text-[14px]">​</p>
                <p className="leading-[1.6] mb-[10px] text-[14px]">
                  Categories of Personal Information in Cal. Civ. Code Section
                  1798.80(e) (Name, address, telephone number, bank account
                  number, credit card number, debit card number, or any other
                  financial information); Characteristics of Protected
                  Classifications under California or Federal Law (Age (over
                  40), sex, including gender); Sensitive personal information
                  (Contents of communication, account access information): We do
                  not share this information for cross-context behavioral
                  advertising.
                </p>
                <p className="leading-[1.6] mb-[10px] text-[14px]">​</p>
                <p className="leading-[1.6] text-[14px]">
                  Inferences Drawn from Personal Information; Biometric
                  Information; Sensory Information; Professional or
                  employment-related information; Non-Public Education
                  Information: We do not collect or process this information.
                </p>
              </div>
            </div>
            <div
              className="content-stretch flex flex-col items-start overflow-clip pl-[24px] relative shrink-0 w-full"
              data-node-id="826:2548"
              data-name="Container / Subchapter 9.5 — Your Rights"
            >
              <div
                className="[word-break:break-word] font-serif font-normal leading-[0] relative shrink-0 text-[#2d241e] text-[0px] w-full whitespace-pre-wrap"
                data-node-id="826:2549"
              >
                <p className="font-serif font-bold leading-[1.6] mb-[10px] text-[16px]">
                  9.5. Your Rights
                </p>
                <p className="leading-[1.6] mb-[10px] text-[14px]">​</p>
                <p className="leading-[1.6] mb-[10px] text-[14px]">
                  Certain U.S. state consumer privacy laws provide you with
                  specific rights regarding your Personal Information. Beyond
                  the rights described in Section 5 of Privacy Policy, you may
                  have the following rights depending on the U.S. state you
                  reside in:
                </p>
                <p className="leading-[1.6] mb-[10px] text-[14px]">​</p>
                <p className="leading-[1.6] mb-[10px] text-[14px]">{`    Right to Opt-Out of Sale, Sharing, and Targeted Advertising. You may opt out of certain uses of your personal information, including “sharing” of your Personal Information as defined under state consumer privacy laws and the use of your Personal Information for targeted advertising. We also recognize and honor browser- or device-based opt-out preference signals such as the Global Privacy Control (GPC). For California residents, this also serves as our California Notice of Right to Opt-out of Sale or Sharing.`}</p>
                <p className="leading-[1.6] mb-[10px] text-[14px]">{`    Right to Appeal. If we deny your request to exercise any of your rights, you may have the right to appeal our decision. To do so, please contact us via a dedicated request form and explain your concerns.`}</p>
                <p className="leading-[1.6] mb-[10px] text-[14px]">​</p>
                <p className="leading-[1.6] mb-[10px] text-[14px]">
                  You can exercise your rights as described in Section 5 of this
                  Policy. Alternatively, you can submit your request via this
                  request form.
                </p>
                <p className="leading-[1.6] mb-[10px] text-[14px]">​</p>
                <p className="leading-[1.6] text-[14px]">
                  We do not discriminate or retaliate against users because they
                  exercise their privacy rights.
                </p>
              </div>
            </div>
            <div
              className="content-stretch flex flex-col items-start overflow-clip pl-[24px] relative shrink-0 w-full"
              data-node-id="826:2550"
              data-name="Container / Subchapter 9.6 — Third Party Tracking"
            >
              <div
                className="[word-break:break-word] font-serif font-normal leading-[0] relative shrink-0 text-[#2d241e] text-[0px] w-full whitespace-pre-wrap"
                data-node-id="826:2551"
              >
                <p className="font-serif font-bold leading-[1.6] mb-[10px] text-[16px]">
                  9.6. Third-Party Tracking
                </p>
                <p className="leading-[1.6] mb-[10px] text-[14px]">​</p>
                <p className="leading-[1.6] mb-[10px] text-[14px]">
                  When you use the Services, including communications with our
                  advisors (psychics), certain aspects of your interactions may
                  be recorded, monitored, or analyzed in accordance with our
                  Terms of Use and this Policy. This may include recording or
                  analyzing your personal data for the purposes of service
                  delivery, research and improvement, quality assurance, and
                  third-party advertising.
                </p>
                <p className="leading-[1.6] mb-[10px] text-[14px]">​</p>
                <p className="leading-[1.6] mb-[10px] text-[14px]">
                  Where required under applicable law, we provide you with
                  notice of such processing and obtain your consent prior to or
                  at the point of collection. By accessing or using our
                  Services, you further acknowledge and consent to such
                  recording, monitoring, and analysis of your communications to
                  the extent permitted by applicable law. To the extent required
                  by applicable law, we hereby notify you that your interactions
                  with our Services may be subject to monitoring, recording, or
                  analysis for the purposes described in this Policy and our
                  Terms of Use, and your continued use of our Services
                  constitutes your explicit consent thereto.
                </p>
                <p className="leading-[1.6] mb-[10px] text-[14px]">​</p>
                <p className="leading-[1.6] text-[14px]">
                  If you do not consent to the processing practices described
                  above, please discontinue use of our Services.
                </p>
              </div>
            </div>
          </div>
          <div
            className="content-stretch flex flex-col items-start overflow-clip relative shrink-0 w-full"
            data-node-id="826:2522"
            data-name="Container / Chapter 10 — SECURITY MEASURES"
          >
            <div
              className="[word-break:break-word] font-serif font-normal leading-[0] relative shrink-0 text-[#2d241e] text-[0px] w-full whitespace-pre-wrap"
              data-node-id="826:2523"
            >
              <h2
                className="font-serif font-bold leading-[1.6] mb-[10px] text-[18px]"
                id="security-measures"
              >
                10. SECURITY MEASURES
              </h2>
              <p className="leading-[1.6] mb-[10px] text-[14px]">​</p>
              <p className="leading-[1.6] text-[14px]">
                We use reasonable and appropriate organisational, technical,
                administrative, and physical security measures as required by
                the applicable laws to protect your personal data from
                unauthorized access, disclosure, use, and modification. This
                includes without limitation pseudonymisation and encryption of
                personal data, access management, vulnerability testing, user
                authorisation procedures, organisational policies, and regular
                tests and assessments of technical and organisational measures.
                We regularly review our security procedures and policies to
                consider appropriate new technology and methods. However, the
                Internet is not 100% secure and we cannot promise that your use
                of the Services of the App will be completely secure. We
                encourage you to use caution when using the Internet, including
                not sharing your passwords to our Services.
              </p>
            </div>
          </div>
          <div
            className="content-stretch flex flex-col items-start overflow-clip relative shrink-0 w-full"
            data-node-id="826:2524"
            data-name="Container / Chapter 11 — THIRD PARTY WEBSITES AND APPLICATIONS"
          >
            <div
              className="[word-break:break-word] font-serif font-normal leading-[0] relative shrink-0 text-[#2d241e] text-[0px] w-full whitespace-pre-wrap"
              data-node-id="826:2525"
            >
              <h2
                className="font-serif font-bold leading-[1.6] mb-[10px] text-[18px]"
                id="third-party-websites-and-applications"
              >
                11. THIRD-PARTY WEBSITES AND APPLICATIONS
              </h2>
              <p className="leading-[1.6] mb-[10px] text-[14px]">​</p>
              <p className="leading-[1.6] text-[14px]">
                The Services may contain links to other websites or applications
                and other websites or applications may reference or link to our
                Services. These third-party services are not controlled by us.
                We encourage our users to read the privacy policies of each
                website and application with which they interact. We do not
                endorse, screen or approve, and are not responsible for, the
                privacy practices or content of such other websites or
                applications. Visiting these other websites or applications is
                at your own risk.
              </p>
            </div>
          </div>
          <div
            className="content-stretch flex flex-col items-start overflow-clip relative shrink-0 w-full"
            data-node-id="826:2526"
            data-name="Container / Chapter 12 — CHANGES TO THIS PRIVACY POLICY"
          >
            <div
              className="[word-break:break-word] font-serif font-normal leading-[0] relative shrink-0 text-[#2d241e] text-[0px] w-full whitespace-pre-wrap"
              data-node-id="826:2527"
            >
              <h2
                className="font-serif font-bold leading-[1.6] mb-[10px] text-[18px]"
                id="changes-to-this-privacy-policy"
              >
                12. CHANGES TO THIS PRIVACY POLICY
              </h2>
              <p className="leading-[1.6] mb-[10px] text-[14px]">​</p>
              <p className="leading-[1.6] text-[14px]">
                We may modify this Privacy Policy from time to time. If we
                decide to make material changes to this Privacy Policy, we will
                notify you through our Services or by other available means and
                will provide you with an opportunity to review the revised
                Privacy Policy. By continuing to access or use the Services or
                the App after those changes become effective, you agree to be
                bound by the revised Privacy Policy.
              </p>
            </div>
          </div>
          <div
            className="content-stretch flex flex-col items-start overflow-clip relative shrink-0 w-full"
            data-node-id="826:2528"
            data-name="Container / Chapter 13 — CONTACT US"
          >
            <div
              className="[word-break:break-word] font-serif font-normal leading-[0] relative shrink-0 text-[#2d241e] text-[0px] w-full whitespace-pre-wrap"
              data-node-id="826:2529"
            >
              <h2
                className="font-serif font-bold leading-[1.6] mb-[10px] text-[18px]"
                id="contact-us"
              >
                13. CONTACT US
              </h2>
              <p className="leading-[1.6] mb-[10px] text-[14px]">​</p>
              <p className="leading-[1.6] mb-[10px] text-[14px]">
                You may contact us at any time for details regarding this
                Privacy Policy and its previous versions. For any questions
                concerning your account or your personal data please contact us
                using this request form.
              </p>
              <p className="leading-[1.6] mb-[10px] text-[14px]">​</p>
              <p className="leading-[1.6] mb-[10px] text-[14px]">
                You can also contact our data protection officer regarding any
                privacy‑ or data‑protection‑related matter using the dedicated
                contact form.
              </p>
              <p className="leading-[1.6] mb-[10px] text-[14px]">​</p>
              <p className="leading-[1.6] text-[14px]">
                Effective as of: October 1, 2026
              </p>
            </div>
          </div>
        </div>
      </div>
    </div>
  );
}

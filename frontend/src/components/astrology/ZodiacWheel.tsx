"use client";
import { useI18n } from "@/i18n/i18n-provider";
/* eslint-disable @next/next/no-img-element */
const imgShapeEllipseAstronomicalChart =
  "https://assets.staryield.net/assets/figma/e2d75.png";
const imgShapeEllipseAstronomicalChart02 =
  "https://assets.staryield.net/assets/figma/41c0a.png";
const imgDecorationStaryieldLogo1 =
  "https://assets.staryield.net/assets/figma/f90ea.png";
const imgGroupStaryieldLogo =
  "https://assets.staryield.net/assets/figma/3e3d0.svg";
const imgShapeEllipseAstronomicalChart03 =
  "https://assets.staryield.net/assets/figma/c6d6d.svg";
const imgShapeEllipseAstronomicalChart04 =
  "https://assets.staryield.net/assets/figma/088fe.svg";
const imgShapeEllipseAstronomicalChart05 =
  "https://assets.staryield.net/assets/figma/ee9a5.svg";
const imgIndicatorDot = "https://assets.staryield.net/assets/figma/8108a.svg";
const imgDividerHorizontal =
  "https://assets.staryield.net/assets/figma/e80ef.svg";
const imgDividerHorizontal02 =
  "https://assets.staryield.net/assets/figma/733d7.svg";
const imgDividerHorizontal03 =
  "https://assets.staryield.net/assets/figma/107bd.svg";
const imgDividerHorizontal04 =
  "https://assets.staryield.net/assets/figma/90991.svg";
const imgDividerHorizontal05 =
  "https://assets.staryield.net/assets/figma/00e44.svg";
const imgDividerHorizontal06 =
  "https://assets.staryield.net/assets/figma/bf4e8.svg";
const imgDividerHorizontal07 =
  "https://assets.staryield.net/assets/figma/ea8b1.svg";
const imgDividerHorizontal08 =
  "https://assets.staryield.net/assets/figma/3ac07.svg";
const imgDividerHorizontal09 =
  "https://assets.staryield.net/assets/figma/12e3a.svg";
const imgDividerHorizontal10 =
  "https://assets.staryield.net/assets/figma/29171.svg";
const imgDividerHorizontal11 =
  "https://assets.staryield.net/assets/figma/f6692.svg";
const imgDividerHorizontal12 =
  "https://assets.staryield.net/assets/figma/bbdfd.svg";
const imgInstanceAries = "https://assets.staryield.net/assets/figma/9a293.svg";
const imgInstanceTaurus = "https://assets.staryield.net/assets/figma/1e525.svg";
const imgInstanceGemini = "https://assets.staryield.net/assets/figma/dd3fa.svg";
const imgInstanceCancer = "https://assets.staryield.net/assets/figma/6e31f.svg";
const imgInstanceLeo = "https://assets.staryield.net/assets/figma/c6980.svg";
const imgInstanceVirgo = "https://assets.staryield.net/assets/figma/341a8.svg";
const imgInstanceLibra = "https://assets.staryield.net/assets/figma/66a4a.svg";
const imgInstanceScorpio =
  "https://assets.staryield.net/assets/figma/7f87b.svg";
const imgInstanceSagittarius =
  "https://assets.staryield.net/assets/figma/87e4b.svg";
const imgInstanceCapricorn =
  "https://assets.staryield.net/assets/figma/12acb.svg";
const imgInstanceAquarius =
  "https://assets.staryield.net/assets/figma/98d0a.svg";
const imgInstancePisces = "https://assets.staryield.net/assets/figma/2f624.svg";
export function ZodiacWheel({ sign = "Gemini" }: { sign?: string }) {
  const { text } = useI18n();
  const names = [
    "Aries",
    "Taurus",
    "Gemini",
    "Cancer",
    "Leo",
    "Virgo",
    "Libra",
    "Scorpio",
    "Sagittarius",
    "Capricorn",
    "Aquarius",
    "Pisces",
  ];
  const positions = [
    [424, 167],
    [518, 222],
    [573, 316],
    [573, 424],
    [518, 518],
    [424, 573],
    [316, 573],
    [222, 518],
    [167, 424],
    [167, 316],
    [222, 222],
    [316, 167],
  ];
  const [left, top] = positions[Math.max(0, names.indexOf(sign))];
  return (
    <div
      className="relative shrink-0 size-[800px]"
      data-node-id="464:11072"
      data-name="Container / Astronomical Chart"
    >
      <div
        className="absolute bg-[var(--warm-gray)] border-3 border-black border-solid left-0 rounded-[400px] size-[800px] top-0"
        data-node-id="464:11073"
        data-name="Shape / Space Matrix"
      />
      <div
        className="-translate-x-1/2 -translate-y-1/2 absolute left-1/2 size-[760px] top-1/2"
        data-node-id="464:11074"
        data-name="Shape / Ellipse / Astronomical Chart"
      >
        <img
          alt=""
          className="absolute block inset-0 max-w-none size-full"
          height="760"
          src={imgShapeEllipseAstronomicalChart}
          width="760"
        />
      </div>
      <div
        className="-translate-x-1/2 -translate-y-1/2 absolute left-1/2 size-[690px] top-1/2"
        data-node-id="464:11075"
        data-name="Shape / Ellipse / Astronomical Chart / 02"
      >
        <img
          alt=""
          className="absolute block inset-0 max-w-none size-full"
          height="690"
          src={imgShapeEllipseAstronomicalChart02}
          width="690"
        />
      </div>
      <div
        className="-translate-x-1/2 -translate-y-1/2 absolute left-1/2 size-[510px] top-1/2"
        data-node-id="464:11076"
        data-name="Shape / Ellipse / Astronomical Chart / 03"
      >
        <img
          alt=""
          className="absolute block inset-0 max-w-none size-full"
          src={imgShapeEllipseAstronomicalChart03}
        />
      </div>
      <div
        className="-translate-x-1/2 -translate-y-1/2 absolute left-1/2 size-[310px] top-1/2"
        data-node-id="464:11077"
        data-name="Shape / Ellipse / Astronomical Chart / 04"
      >
        <img
          alt=""
          className="absolute block inset-0 max-w-none size-full"
          src={imgShapeEllipseAstronomicalChart04}
        />
      </div>
      <div
        className="-translate-x-1/2 -translate-y-1/2 absolute left-1/2 size-[170px] top-1/2"
        data-node-id="464:11078"
        data-name="Shape / Ellipse / Astronomical Chart / 05"
      >
        <img
          alt=""
          className="absolute block inset-0 max-w-none size-full"
          src={imgShapeEllipseAstronomicalChart05}
        />
      </div>
      <div
        className="absolute left-[620px] size-[2px] top-[310px]"
        data-node-id="464:11083"
        data-name="Indicator / Dot"
      >
        <img
          alt=""
          className="absolute block inset-0 max-w-none size-full"
          src={imgIndicatorDot}
        />
      </div>
      <div
        className="absolute flex h-[345px] items-center justify-center left-[400px] top-[55px] w-0"
        data-node-id="464:11090"
      >
        <div className="-rotate-90 flex-none">
          <div
            className="h-0 relative w-[345px]"
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
        </div>
      </div>
      <div
        className="absolute flex h-[298.779px] items-center justify-center left-[400px] top-[101.22px] w-[172.5px]"
        data-node-id="464:11091"
      >
        <div className="-rotate-60 flex-none">
          <div
            className="h-0 relative w-[345px]"
            data-name="Divider / Horizontal / 02"
          >
            <div className="absolute inset-[-1px_0_0_0]">
              <img
                alt=""
                className="block max-w-none size-full"
                src={imgDividerHorizontal02}
              />
            </div>
          </div>
        </div>
      </div>
      <div
        className="absolute flex h-[172.5px] items-center justify-center left-[400px] top-[227.5px] w-[298.779px]"
        data-node-id="464:11092"
      >
        <div className="-rotate-30 flex-none">
          <div
            className="h-0 relative w-[345px]"
            data-name="Divider / Horizontal / 03"
          >
            <div className="absolute inset-[-1px_0_0_0]">
              <img
                alt=""
                className="block max-w-none size-full"
                src={imgDividerHorizontal03}
              />
            </div>
          </div>
        </div>
      </div>
      <div
        className="absolute h-0 left-[400px] top-[400px] w-[345px]"
        data-node-id="464:11093"
        data-name="Divider / Horizontal / 04"
      >
        <div className="absolute inset-[-1px_0_0_0]">
          <img
            alt=""
            className="block max-w-none size-full"
            src={imgDividerHorizontal04}
          />
        </div>
      </div>
      <div
        className="absolute flex h-[172.5px] items-center justify-center left-[400px] top-[400px] w-[298.779px]"
        data-node-id="464:11094"
      >
        <div className="flex-none rotate-30">
          <div
            className="h-0 relative w-[345px]"
            data-name="Divider / Horizontal / 05"
          >
            <div className="absolute inset-[-1px_0_0_0]">
              <img
                alt=""
                className="block max-w-none size-full"
                src={imgDividerHorizontal05}
              />
            </div>
          </div>
        </div>
      </div>
      <div
        className="absolute flex h-[298.779px] items-center justify-center left-[400px] top-[400px] w-[172.5px]"
        data-node-id="464:11095"
      >
        <div className="flex-none rotate-60">
          <div
            className="h-0 relative w-[345px]"
            data-name="Divider / Horizontal / 06"
          >
            <div className="absolute inset-[-1px_0_0_0]">
              <img
                alt=""
                className="block max-w-none size-full"
                src={imgDividerHorizontal06}
              />
            </div>
          </div>
        </div>
      </div>
      <div
        className="absolute flex h-[345px] items-center justify-center left-[400px] top-[400px] w-0"
        data-node-id="464:11096"
      >
        <div className="flex-none rotate-90">
          <div
            className="h-0 relative w-[345px]"
            data-name="Divider / Horizontal / 07"
          >
            <div className="absolute inset-[-1px_0_0_0]">
              <img
                alt=""
                className="block max-w-none size-full"
                src={imgDividerHorizontal07}
              />
            </div>
          </div>
        </div>
      </div>
      <div
        className="absolute flex h-[298.779px] items-center justify-center left-[227.5px] top-[400px] w-[172.5px]"
        data-node-id="464:11097"
      >
        <div className="flex-none rotate-120">
          <div
            className="h-0 relative w-[345px]"
            data-name="Divider / Horizontal / 08"
          >
            <div className="absolute inset-[-1px_0_0_0]">
              <img
                alt=""
                className="block max-w-none size-full"
                src={imgDividerHorizontal08}
              />
            </div>
          </div>
        </div>
      </div>
      <div
        className="absolute flex h-[172.5px] items-center justify-center left-[101.22px] top-[400px] w-[298.779px]"
        data-node-id="464:11098"
      >
        <div className="flex-none rotate-150">
          <div
            className="h-0 relative w-[345px]"
            data-name="Divider / Horizontal / 09"
          >
            <div className="absolute inset-[-1px_0_0_0]">
              <img
                alt=""
                className="block max-w-none size-full"
                src={imgDividerHorizontal09}
              />
            </div>
          </div>
        </div>
      </div>
      <div
        className="absolute flex h-0 items-center justify-center left-[55px] top-[400px] w-[345px]"
        data-node-id="464:11099"
      >
        <div className="flex-none rotate-180">
          <div
            className="h-0 relative w-[345px]"
            data-name="Divider / Horizontal / 10"
          >
            <div className="absolute inset-[-1px_0_0_0]">
              <img
                alt=""
                className="block max-w-none size-full"
                src={imgDividerHorizontal10}
              />
            </div>
          </div>
        </div>
      </div>
      <div
        className="absolute flex h-[172.5px] items-center justify-center left-[101.22px] top-[227.5px] w-[298.779px]"
        data-node-id="464:11100"
      >
        <div className="-rotate-150 flex-none">
          <div
            className="h-0 relative w-[345px]"
            data-name="Divider / Horizontal / 11"
          >
            <div className="absolute inset-[-1px_0_0_0]">
              <img
                alt=""
                className="block max-w-none size-full"
                src={imgDividerHorizontal11}
              />
            </div>
          </div>
        </div>
      </div>
      <div
        className="absolute flex h-[298.779px] items-center justify-center left-[227.5px] top-[101.22px] w-[172.5px]"
        data-node-id="464:11101"
      >
        <div className="-rotate-120 flex-none">
          <div
            className="h-0 relative w-[345px]"
            data-name="Divider / Horizontal / 12"
          >
            <div className="absolute inset-[-1px_0_0_0]">
              <img
                alt=""
                className="block max-w-none size-full"
                src={imgDividerHorizontal12}
              />
            </div>
          </div>
        </div>
      </div>
      <div
        aria-hidden="true"
        className="absolute size-[84px] rounded-full border-[2.5px] border-black bg-amber shadow-[0_0_16px_#c29a3d]"
        style={{ left: left - 12, top: top - 12 }}
      />
      <div
        className="absolute left-0 size-[800px] top-0"
        data-node-id="464:11102"
        data-name="Layout / Astronomical Chart"
      >
        <div
          className="absolute content-stretch flex h-[30px] items-center justify-center left-[401px] top-[100px] w-[150px]"
          data-node-id="464:11103"
          data-name="Control / ARIES"
        >
          <div
            className="flex h-[27.809px] items-center justify-center relative shrink-0 w-[46.901px]"
            data-node-id="464:11104"
          >
            <div className="flex-none rotate-15">
              <p className="[word-break:break-word] font-display leading-[normal] not-italic relative text-[14px] text-black text-center tracking-[0.42px] whitespace-nowrap">
                {text("ARIES ")}
              </p>
            </div>
          </div>
        </div>
        <div
          className="absolute content-stretch flex items-center justify-center left-[424px] size-[60px] top-[167px]"
          data-node-id="464:11105"
          data-name="Layout / Astronomical Chart"
        >
          <div
            className="h-[51px] relative shrink-0 w-[50.653px]"
            data-node-id="464:11106"
            data-name="Instance / Aries"
          >
            <img
              alt=""
              className="absolute block inset-0 max-w-none size-full"
              src={imgInstanceAries}
            />
          </div>
        </div>
      </div>
      <div
        className="absolute left-0 size-[800px] top-0"
        data-node-id="464:11107"
        data-name="Layout / Astronomical Chart / 02"
      >
        <div
          className="absolute content-stretch flex h-[30px] items-center justify-center left-[534px] top-[176px] w-[150px]"
          data-node-id="464:11108"
          data-name="Control / TAURUS"
        >
          <div
            className="flex items-center justify-center relative shrink-0 size-[57.983px]"
            data-node-id="464:11109"
          >
            <div className="flex-none rotate-45">
              <p className="[word-break:break-word] font-display leading-[normal] not-italic relative text-[14px] text-black text-center tracking-[0.42px] whitespace-nowrap">
                {text("TAURUS ")}
              </p>
            </div>
          </div>
        </div>
        <div
          className="absolute content-stretch flex items-center justify-center left-[518px] size-[60px] top-[222px]"
          data-node-id="464:11110"
          data-name="Layout / 02"
        >
          <div
            className="h-[51px] relative shrink-0 w-[45.447px]"
            data-node-id="464:11111"
            data-name="Instance / Taurus"
          >
            <img
              alt=""
              className="absolute block inset-0 max-w-none size-full"
              src={imgInstanceTaurus}
            />
          </div>
        </div>
      </div>
      <div
        className="absolute left-0 size-[800px] top-0"
        data-node-id="464:11112"
        data-name="Layout / Astronomical Chart / 03"
      >
        <div
          className="absolute content-stretch flex items-center justify-center left-[573px] size-[60px] top-[316px]"
          data-node-id="464:11114"
          data-name="Layout / 03"
        >
          <div
            className="h-[51px] relative shrink-0 w-[46.995px]"
            data-node-id="464:11115"
            data-name="Instance / Gemini"
          >
            <img
              alt=""
              className="absolute block inset-0 max-w-none size-full"
              src={imgInstanceGemini}
            />
          </div>
        </div>
        <div
          className="absolute content-stretch flex h-[30px] items-center justify-center left-[610px] top-[309px] w-[150px]"
          data-node-id="464:11116"
          data-name="Control / GEMINI"
        >
          <div
            className="flex h-[62.355px] items-center justify-center relative shrink-0 w-[31.95px]"
            data-node-id="464:11117"
          >
            <div className="flex-none rotate-75">
              <p className="[word-break:break-word] font-display leading-[normal] not-italic relative text-[14px] text-black text-center tracking-[0.42px] whitespace-nowrap">
                {text("GEMINI ")}
              </p>
            </div>
          </div>
        </div>
      </div>
      <div
        className="absolute left-0 size-[800px] top-0"
        data-node-id="464:11118"
        data-name="Layout / Astronomical Chart / 04"
      >
        <div
          className="absolute content-stretch flex items-center justify-center left-[573px] size-[60px] top-[424px]"
          data-node-id="464:11119"
          data-name="Layout / 04"
        >
          <div
            className="h-[42.506px] relative shrink-0 w-[51px]"
            data-node-id="464:11120"
            data-name="Instance / Cancer"
          >
            <img
              alt=""
              className="absolute block inset-0 max-w-none size-full"
              src={imgInstanceCancer}
            />
          </div>
        </div>
        <div
          className="absolute content-stretch flex h-[30px] items-center justify-center left-[610px] top-[461px] w-[150px]"
          data-node-id="464:11121"
          data-name="Control / CANCER"
        >
          <div
            className="flex h-[69.117px] items-center justify-center relative shrink-0 w-[33.762px]"
            data-node-id="464:11122"
          >
            <div className="flex-none rotate-105">
              <p className="[word-break:break-word] font-display leading-[normal] not-italic relative text-[14px] text-black text-center tracking-[0.42px] whitespace-nowrap">
                {text("CANCER ")}
              </p>
            </div>
          </div>
        </div>
      </div>
      <div
        className="absolute left-0 size-[800px] top-0"
        data-node-id="464:11123"
        data-name="Layout / Astronomical Chart / 05"
      >
        <div
          className="absolute content-stretch flex items-center justify-center left-[518px] size-[60px] top-[518px]"
          data-node-id="464:11124"
          data-name="Layout / 05"
        >
          <div
            className="h-[51px] relative shrink-0 w-[37.399px]"
            data-node-id="464:11125"
            data-name="Instance / Leo"
          >
            <img
              alt=""
              className="absolute block inset-0 max-w-none size-full"
              src={imgInstanceLeo}
            />
          </div>
        </div>
        <div
          className="absolute content-stretch flex h-[30px] items-center justify-center left-[534px] top-[594px] w-[150px]"
          data-node-id="464:11126"
          data-name="Control / LEO"
        >
          <div
            className="flex items-center justify-center relative shrink-0 size-[33.234px]"
            data-node-id="464:11127"
          >
            <div className="flex-none rotate-135">
              <p className="[word-break:break-word] font-display leading-[normal] not-italic relative text-[14px] text-black text-center tracking-[0.42px] whitespace-nowrap">
                {text("LEO ")}
              </p>
            </div>
          </div>
        </div>
      </div>
      <div
        className="absolute left-0 size-[800px] top-0"
        data-node-id="464:11128"
        data-name="Layout / Astronomical Chart / 06"
      >
        <div
          className="absolute content-stretch flex items-center justify-center left-[424px] size-[60px] top-[573px]"
          data-node-id="464:11129"
          data-name="Layout / 06"
        >
          <div
            className="h-[51px] relative shrink-0 w-[40.025px]"
            data-node-id="464:11130"
            data-name="Instance / Virgo"
          >
            <img
              alt=""
              className="absolute block inset-0 max-w-none size-full"
              src={imgInstanceVirgo}
            />
          </div>
        </div>
        <div
          className="absolute content-stretch flex h-[30px] items-center justify-center left-[401px] top-[670px] w-[150px]"
          data-node-id="464:11131"
          data-name="Control / VIRGO"
        >
          <div
            className="flex h-[30.138px] items-center justify-center relative shrink-0 w-[55.594px]"
            data-node-id="464:11132"
          >
            <div className="flex-none rotate-165">
              <p className="[word-break:break-word] font-display leading-[normal] not-italic relative text-[14px] text-black text-center tracking-[0.42px] whitespace-nowrap">
                {text("VIRGO ")}
              </p>
            </div>
          </div>
        </div>
      </div>
      <div
        className="absolute left-0 size-[800px] top-0"
        data-node-id="464:11133"
        data-name="Layout / Astronomical Chart / 07"
      >
        <div
          className="absolute content-stretch flex items-center justify-center left-[316px] size-[60px] top-[573px]"
          data-node-id="464:11134"
          data-name="Layout / 07"
        >
          <div
            className="h-[45.298px] relative shrink-0 w-[51px]"
            data-node-id="464:11135"
            data-name="Instance / Libra"
          >
            <img
              alt=""
              className="absolute block inset-0 max-w-none size-full"
              src={imgInstanceLibra}
            />
          </div>
        </div>
        <div
          className="absolute content-stretch flex h-[30px] items-center justify-center left-[249px] top-[670px] w-[150px]"
          data-node-id="464:11136"
          data-name="Control / LIBRA"
        >
          <div
            className="flex h-[28.794px] items-center justify-center relative shrink-0 w-[50.76px]"
            data-node-id="464:11137"
          >
            <div className="flex-none rotate-[-165.07deg]">
              <p className="[word-break:break-word] font-display leading-[normal] not-italic relative text-[14px] text-black text-center tracking-[0.42px] whitespace-nowrap">
                {text("LIBRA ")}
              </p>
            </div>
          </div>
        </div>
      </div>
      <div
        className="absolute left-0 size-[800px] top-0"
        data-node-id="464:11138"
        data-name="Layout / Astronomical Chart / 08"
      >
        <div
          className="absolute content-stretch flex items-center justify-center left-[222px] size-[60px] top-[518px]"
          data-node-id="464:11139"
          data-name="Layout / 08"
        >
          <div
            className="h-[51px] relative shrink-0 w-[42.579px]"
            data-node-id="464:11140"
            data-name="Instance / Scorpio"
          >
            <img
              alt=""
              className="absolute block inset-0 max-w-none size-full"
              src={imgInstanceScorpio}
            />
          </div>
        </div>
        <div
          className="absolute content-stretch flex h-[30px] items-center justify-center left-[116px] top-[594px] w-[150px]"
          data-node-id="464:11141"
          data-name="Control / SCORPIO"
        >
          <div
            className="flex items-center justify-center relative shrink-0 size-[61.518px]"
            data-node-id="464:11142"
          >
            <div className="-rotate-135 flex-none">
              <p className="[word-break:break-word] font-display leading-[normal] not-italic relative text-[14px] text-black text-center tracking-[0.42px] whitespace-nowrap">
                {text("SCORPIO ")}
              </p>
            </div>
          </div>
        </div>
      </div>
      <div
        className="absolute left-0 size-[800px] top-0"
        data-node-id="464:11143"
        data-name="Layout / Astronomical Chart / 09"
      >
        <div
          className="absolute content-stretch flex items-center justify-center left-[167px] size-[60px] top-[424px]"
          data-node-id="464:11144"
          data-name="Layout / 09"
        >
          <div
            className="h-[51px] relative shrink-0 w-[48.093px]"
            data-node-id="464:11145"
            data-name="Instance / Sagittarius"
          >
            <img
              alt=""
              className="absolute block inset-0 max-w-none size-full"
              src={imgInstanceSagittarius}
            />
          </div>
        </div>
        <div
          className="absolute content-stretch flex h-[30px] items-center justify-center left-[40px] top-[461px] w-[150px]"
          data-node-id="464:11146"
          data-name="Control / SAGITTARIUS"
        >
          <div
            className="flex h-[105.835px] items-center justify-center relative shrink-0 w-[43.481px]"
            data-node-id="464:11147"
          >
            <div className="flex-none rotate-[-104.93deg]">
              <p className="[word-break:break-word] font-display leading-[normal] not-italic relative text-[14px] text-black text-center tracking-[0.42px] whitespace-nowrap">
                {text("SAGITTARIUS ")}
              </p>
            </div>
          </div>
        </div>
      </div>
      <div
        className="absolute left-0 size-[800px] top-0"
        data-node-id="464:11148"
        data-name="Layout / Astronomical Chart / 10"
      >
        <div
          className="absolute content-stretch flex items-center justify-center left-[167px] size-[60px] top-[316px]"
          data-node-id="464:11149"
          data-name="Layout / 10"
        >
          <div
            className="h-[51px] relative shrink-0 w-[50.302px]"
            data-node-id="464:11150"
            data-name="Instance / Capricorn"
          >
            <img
              alt=""
              className="absolute block inset-0 max-w-none size-full"
              src={imgInstanceCapricorn}
            />
          </div>
        </div>
        <div
          className="absolute content-stretch flex h-[30px] items-center justify-center left-[40px] top-[309px] w-[150px]"
          data-node-id="464:11151"
          data-name="Control / CAPRICORN"
        >
          <div
            className="flex h-[98.105px] items-center justify-center relative shrink-0 w-[41.419px]"
            data-node-id="464:11152"
          >
            <div className="flex-none rotate-[-75.07deg]">
              <p className="[word-break:break-word] font-display leading-[normal] not-italic relative text-[14px] text-black text-center tracking-[0.42px] whitespace-nowrap">
                {text("CAPRICORN ")}
              </p>
            </div>
          </div>
        </div>
      </div>
      <div
        className="absolute left-0 size-[800px] top-0"
        data-node-id="464:11153"
        data-name="Layout / Astronomical Chart / 11"
      >
        <div
          className="absolute content-stretch flex h-[30px] items-center justify-center left-[116px] top-[176px] w-[150px]"
          data-node-id="464:11154"
          data-name="Control / AQUARIUS"
        >
          <div
            className="flex items-center justify-center relative shrink-0 size-[72.832px]"
            data-node-id="464:11155"
          >
            <div className="-rotate-45 flex-none">
              <p className="[word-break:break-word] font-display leading-[normal] not-italic relative text-[14px] text-black text-center tracking-[0.42px] whitespace-nowrap">
                {text("AQUARIUS ")}
              </p>
            </div>
          </div>
        </div>
        <div
          className="absolute content-stretch flex items-center justify-center left-[222px] size-[60px] top-[222px]"
          data-node-id="464:11156"
          data-name="Layout / 11"
        >
          <div
            className="h-[36.42px] relative shrink-0 w-[51px]"
            data-node-id="464:11157"
            data-name="Instance / Aquarius"
          >
            <img
              alt=""
              className="absolute block inset-0 max-w-none size-full"
              src={imgInstanceAquarius}
            />
          </div>
        </div>
      </div>
      <div
        className="absolute left-0 size-[800px] top-0"
        data-node-id="464:11158"
        data-name="Layout / Astronomical Chart / 12"
      >
        <div
          className="absolute content-stretch flex h-[30px] items-center justify-center left-[249px] top-[100px] w-[150px]"
          data-node-id="464:11159"
          data-name="Control / PISCES"
        >
          <div
            className="flex h-[29.051px] items-center justify-center relative shrink-0 w-[51.726px]"
            data-node-id="464:11160"
          >
            <div className="flex-none rotate-[-14.93deg]">
              <p className="[word-break:break-word] font-display leading-[normal] not-italic relative text-[14px] text-black text-center tracking-[0.42px] whitespace-nowrap">
                {text("PISCES ")}
              </p>
            </div>
          </div>
        </div>
        <div
          className="absolute content-stretch flex items-center justify-center left-[316px] size-[60px] top-[167px]"
          data-node-id="464:11161"
          data-name="Layout / 12"
        >
          <div
            className="h-[51px] relative shrink-0 w-[38.557px]"
            data-node-id="464:11162"
            data-name="Instance / Pisces"
          >
            <img
              alt=""
              className="absolute block inset-0 max-w-none size-full"
              src={imgInstancePisces}
            />
          </div>
        </div>
      </div>
      <div
        className="-translate-x-1/2 -translate-y-1/2 absolute content-stretch flex items-center justify-center left-1/2 size-[120px] top-1/2"
        data-node-id="464:11163"
        data-name="Container / Sun Core Star"
      >
        <div
          className="overflow-clip relative shrink-0 size-[100px]"
          data-node-id="464:11164"
          data-name="Instance / Staryield Logo"
        >
          <div
            className="absolute inset-[0.33%]"
            data-node-id="I464:11164;21:3"
            data-name="Decoration / Staryield Logo"
          >
            <div className="absolute inset-[-0.75%]">
              <img
                alt=""
                className="block max-w-none size-full"
                height="100.84"
                src={imgDecorationStaryieldLogo1}
                width="100.84"
              />
            </div>
          </div>
          <div
            className="absolute inset-[37.68%_30.8%_31.48%_30.42%]"
            data-node-id="I464:11164;21:4"
            data-name="Group / Staryield Logo"
          >
            <img
              alt=""
              className="absolute block inset-0 max-w-none size-full"
              src={imgGroupStaryieldLogo}
            />
          </div>
        </div>
      </div>
    </div>
  );
}

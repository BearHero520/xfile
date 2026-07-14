import { CloudArrowUp24Filled } from "@fluentui/react-icons";
import { useEffect, useState } from "react";
import { useApp } from "../state";

export default function BrandMark() {
  const { site } = useApp();
  const logoUrl = site?.preferences.brandLogoUrl || "";
  const [failed, setFailed] = useState(false);

  useEffect(() => setFailed(false), [logoUrl]);

  return (
    <span
      className={`brand-mark ${logoUrl && !failed ? "has-image" : ""}`}
      aria-hidden="true"
    >
      {logoUrl && !failed ? (
        <img src={logoUrl} alt="" onError={() => setFailed(true)} />
      ) : (
        <CloudArrowUp24Filled />
      )}
    </span>
  );
}

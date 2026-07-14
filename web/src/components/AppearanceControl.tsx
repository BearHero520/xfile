import {
  WeatherMoon20Regular,
  WeatherSunny20Regular,
} from "@fluentui/react-icons";
import {
  type MotionPreference,
  type RadiusPreference,
  type ThemePreference,
  useAppearance,
} from "../appearance";
import { IconButton, Popover, SegmentedControl } from "./ui";

const themeOptions: Array<{ value: ThemePreference; label: string }> = [
  { value: "system", label: "系统" },
  { value: "light", label: "浅色" },
  { value: "dark", label: "深色" },
];

const radiusOptions: Array<{ value: RadiusPreference; label: string }> = [
  { value: "compact", label: "紧凑" },
  { value: "balanced", label: "标准" },
  { value: "rounded", label: "圆润" },
];

const motionOptions: Array<{ value: MotionPreference; label: string }> = [
  { value: "reduced", label: "减少" },
  { value: "standard", label: "标准" },
  { value: "expressive", label: "增强" },
];

export default function AppearanceControl() {
  const {
    theme,
    resolvedTheme,
    radius,
    motion,
    setTheme,
    setRadius,
    setMotion,
  } = useAppearance();

  return (
    <Popover
      label="外观设置"
      className="appearance-control"
      trigger={({ open, toggle }) => (
        <IconButton
          label="外观设置"
          variant="surface"
          active={open}
          onClick={toggle}
        >
          {resolvedTheme === "dark" ? (
            <WeatherMoon20Regular />
          ) : (
            <WeatherSunny20Regular />
          )}
        </IconButton>
      )}
    >
      <header className="appearance-heading">
        <strong>界面外观</strong>
        <small>设置会保存在当前浏览器</small>
      </header>
      <div className="appearance-option">
        <span>主题模式</span>
        <SegmentedControl
          label="主题模式"
          value={theme}
          options={themeOptions}
          onChange={setTheme}
        />
      </div>
      <div className="appearance-option">
        <span>圆角风格</span>
        <SegmentedControl
          label="圆角风格"
          value={radius}
          options={radiusOptions}
          onChange={setRadius}
        />
      </div>
      <div className="appearance-option">
        <span>动效强度</span>
        <SegmentedControl
          label="动效强度"
          value={motion}
          options={motionOptions}
          onChange={setMotion}
        />
      </div>
    </Popover>
  );
}

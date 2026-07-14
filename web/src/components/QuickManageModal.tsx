import type { FavoriteEntry } from "../types";
import {
  Link20Regular,
  Share20Regular,
  Star20Regular,
} from "@fluentui/react-icons";
import DeliveryPage from "../pages/DeliveryPage";
import FavoritesPage from "../pages/FavoritesPage";
import SharesPage from "../pages/SharesPage";
import { Modal } from "./ui";

export type QuickManageSection = "shares" | "delivery" | "favorites";

const sections: Array<{
  key: QuickManageSection;
  label: string;
  icon: React.ReactNode;
}> = [
  { key: "shares", label: "我的分享", icon: <Share20Regular /> },
  { key: "delivery", label: "我的短链", icon: <Link20Regular /> },
  { key: "favorites", label: "我的收藏", icon: <Star20Regular /> },
];

export default function QuickManageModal({
  section,
  onSectionChange,
  onClose,
  onLocateFavorite,
}: {
  section: QuickManageSection | null;
  onSectionChange: (section: QuickManageSection) => void;
  onClose: () => void;
  onLocateFavorite: (item: FavoriteEntry) => void;
}) {
  const activeSection = sections.find((item) => item.key === section);

  return (
    <Modal
      open={section !== null}
      title="快捷管理"
      description={
        activeSection
          ? `当前：${activeSection.label}。关闭后仍停留在原文件目录。`
          : undefined
      }
      size="large"
      className="modal-quick-manage"
      bodyClassName="modal-quick-manage-body"
      onClose={onClose}
    >
      <nav className="quick-manage-tabs" role="tablist" aria-label="快捷管理">
        {sections.map((item) => (
          <button
            key={item.key}
            type="button"
            role="tab"
            aria-selected={section === item.key}
            className={section === item.key ? "is-active" : ""}
            onClick={() => onSectionChange(item.key)}
          >
            {item.icon}
            <span>{item.label}</span>
          </button>
        ))}
      </nav>
      <div className="quick-manage-content" role="tabpanel">
        {section === "shares" && <SharesPage embedded />}
        {section === "delivery" && <DeliveryPage embedded />}
        {section === "favorites" && (
          <FavoritesPage embedded onLocate={onLocateFavorite} />
        )}
      </div>
    </Modal>
  );
}

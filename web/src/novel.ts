export type NovelChapter = {
  title: string;
  line: number;
  bodyStart: number;
  bodyEnd: number;
};

export type NovelDocument = {
  introEnd: number;
  chapters: NovelChapter[];
};

export const novelExtensions = new Set(["txt", "md", "mdx"]);

const chineseChapterPattern =
  /^第[〇零一二三四五六七八九十百千万两\d]{1,12}([章节卷篇部集回])(.*)$/;
const englishChapterPattern = /^(?:chapter|part)\s+[\divxlcdm]+(.*)$/i;
const specialChapterPattern =
  /^(?:序章|楔子|引子|前言|后记|尾声|终章|大结局|番外)(?:\s*[一二三四五六七八九十\d]+)?(.*)$/;

function hasSeparatedTitle(suffix: string) {
  return (
    suffix.length === 0 ||
    /^\s+\S/.test(suffix) ||
    /^\s*[：:、.．\-—]\s*\S/.test(suffix)
  );
}

function isNovelChapterTitle(title: string) {
  const chineseMatch = title.match(chineseChapterPattern);
  if (chineseMatch) {
    const [, unit, suffix] = chineseMatch;
    if (hasSeparatedTitle(suffix)) return true;
    return (
      ["章", "节", "回"].includes(unit) && !/^[的是了，。、“”《]/.test(suffix)
    );
  }

  const englishMatch = title.match(englishChapterPattern);
  if (englishMatch) return hasSeparatedTitle(englishMatch[1]);

  const specialMatch = title.match(specialChapterPattern);
  if (!specialMatch) return false;
  const suffix = specialMatch[1];
  return hasSeparatedTitle(suffix) || !/^[的是了，。、“”《]/.test(suffix);
}

export function detectNovelDocument(
  value: string,
  ext: string,
): NovelDocument | null {
  if (!novelExtensions.has(ext)) return null;

  const headings: Array<{
    line: number;
    title: string;
    headingStart: number;
    bodyStart: number;
  }> = [];
  const linePattern = /([^\r\n]*)(?:\r\n|\n|\r|$)/g;
  let line = 0;
  let match: RegExpExecArray | null;

  while ((match = linePattern.exec(value)) && match.index < value.length) {
    const title = match[1]
      .trim()
      .replace(/^#{1,6}\s*/, "")
      .trim();
    if (title.length > 0 && title.length <= 80 && isNovelChapterTitle(title)) {
      headings.push({
        line,
        title,
        headingStart: match.index,
        bodyStart: linePattern.lastIndex,
      });
    }
    line += 1;
  }

  if (headings.length < 2) return null;
  return {
    introEnd: headings[0].headingStart,
    chapters: headings.map((heading, index) => ({
      title: heading.title,
      line: heading.line,
      bodyStart: heading.bodyStart,
      bodyEnd: headings[index + 1]?.headingStart ?? value.length,
    })),
  };
}

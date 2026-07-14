import { detectNovelDocument, type NovelDocument } from "../novel";

type NovelParserRequest = {
  value: string;
  ext: string;
};

type NovelParserResponse = {
  document: NovelDocument | null;
};

type WorkerScope = {
  onmessage: ((event: MessageEvent<NovelParserRequest>) => void) | null;
  postMessage: (message: NovelParserResponse) => void;
};

const workerScope = globalThis as unknown as WorkerScope;

workerScope.onmessage = (event) => {
  workerScope.postMessage({
    document: detectNovelDocument(event.data.value, event.data.ext),
  });
};

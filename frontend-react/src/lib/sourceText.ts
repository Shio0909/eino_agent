const suspiciousMojibakePattern = /[�ÃÂÐÑÄÅÆÇÈÉÊËÌÍÎÏÒÓÔÕÖØÙÚÛÜÝÞßàáâãäåæçèéêëìíîïðñòóôõöøùúûüýþÿ¤¦§¨©ª«¬®¯°±²³´µ¶·¸¹º»¼½¾¿]/;

export function isLikelyGarbledSourceText(value: string) {
  const text = value.trim();
  if (text.length < 24) return false;

  const chars = Array.from(text).filter((char) => !/\s/.test(char));
  if (chars.length === 0) return false;

  const suspiciousCount = chars.filter((char) => suspiciousMojibakePattern.test(char) || isUnexpectedControlChar(char)).length;
  const symbolCount = chars.filter((char) => /[^\p{L}\p{N}\p{Script=Han}，。！？；：、“”‘’（）《》【】\[\]{}()<>.,!?;:'"`~\-_/\\|@#$%^&*+=\s]/u.test(char)).length;

  return suspiciousCount / chars.length > 0.08 || symbolCount / chars.length > 0.18;
}

function isUnexpectedControlChar(char: string) {
  const code = char.charCodeAt(0);
  return code < 32 && char !== '\n' && char !== '\r' && char !== '\t';
}

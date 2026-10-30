package iped.engine.util;

import static org.junit.Assert.assertEquals;

import java.io.Reader;

import org.junit.After;
import org.junit.Test;

public class TextCacheTest {

    @After
    public void restoreDefault() {
        TextCache.setMaxMemoryChars(TextCache.DEFAULT_MAX_MEMORY_CHARS);
    }

    private static String readAll(TextCache cache) throws Exception {
        StringBuilder sb = new StringBuilder();
        try (Reader reader = cache.getTextReader()) {
            char[] buf = new char[4096];
            int n;
            while ((n = reader.read(buf)) != -1) {
                sb.append(buf, 0, n);
            }
        }
        return sb.toString();
    }

    @Test
    public void lowerLimitWithDiskCacheKeepsAllText() throws Exception {
        TextCache.setMaxMemoryChars(100);
        String text = "0123456789".repeat(100);
        try (TextCache cache = new TextCache()) {
            cache.setEnableDiskCache(true);
            cache.write(text);
            assertEquals(text.length(), cache.getSize());
            assertEquals(text, readAll(cache));
        }
    }

    @Test
    public void lowerLimitWithoutDiskCacheIsIgnored() throws Exception {
        TextCache.setMaxMemoryChars(100);
        String text = "0123456789".repeat(100);
        try (TextCache cache = new TextCache()) {
            cache.setEnableDiskCache(false);
            cache.write(text);
            assertEquals(text, readAll(cache));
        }
    }

    @Test
    public void withoutDiskCacheTextIsStillCappedAtDefault() throws Exception {
        String chunk = "x".repeat(1000000);
        try (TextCache cache = new TextCache()) {
            cache.setEnableDiskCache(false);
            for (int i = 0; i < 12; i++) {
                cache.write(chunk);
            }
            assertEquals(TextCache.DEFAULT_MAX_MEMORY_CHARS, readAll(cache).length());
        }
    }

    @Test
    public void nonPositiveLimitFallsBackToDefault() throws Exception {
        TextCache.setMaxMemoryChars(0);
        String text = "abc".repeat(1000);
        try (TextCache cache = new TextCache()) {
            cache.write(text);
            assertEquals(text, readAll(cache));
        }
    }
}

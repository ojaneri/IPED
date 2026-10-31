package iped.engine.mcp;

import java.io.BufferedReader;
import java.io.File;
import java.io.FileInputStream;
import java.io.InputStreamReader;
import java.io.StringWriter;
import java.nio.charset.StandardCharsets;
import java.util.ArrayList;
import java.util.Arrays;
import java.util.Collections;
import java.util.List;
import java.util.Map;

import org.apache.lucene.document.Document;
import org.apache.lucene.index.IndexableField;
import org.apache.tika.io.TikaInputStream;
import org.apache.tika.metadata.Metadata;
import org.apache.tika.parser.ParseContext;
import org.apache.tika.sax.ToTextContentHandler;
import org.json.simple.JSONArray;
import org.json.simple.JSONObject;

import iped.data.IItem;
import iped.engine.config.Configuration;
import iped.engine.data.IPEDSource;
import iped.engine.search.IPEDSearcher;
import iped.engine.task.ParsingTask;
import iped.parsers.standard.StandardParser;
import iped.search.SearchResult;

public class McpToolHandler {

    private IPEDSource currentSource;
    private File currentCaseDir;

    private Process activeProcess;
    private long activeProcessStartMs;
    private File activeStdoutLog;
    private File activeStderrLog;
    private String activeEvidencePath;
    private String activeOutputPath;

    public synchronized String openCase(String path) {
        try {
            File dir = new File(path);
            if (!dir.exists() || !dir.isDirectory()) {
                return errorJson("Directory not found: " + path);
            }

            File ipedSubdir = new File(dir, "iped");
            if (ipedSubdir.exists()) {
                Configuration.getInstance().loadConfigurables(ipedSubdir.getAbsolutePath(), true);
            }

            if (currentSource != null) {
                try {
                    currentSource.close();
                } catch (Exception ignored) {
                }
            }

            currentSource = new IPEDSource(dir);
            currentCaseDir = dir;

            JSONObject res = new JSONObject();
            res.put("status", "ok");
            res.put("case_dir", dir.getAbsolutePath());
            res.put("total_items", currentSource.getTotalItems());
            res.put("num_docs", currentSource.getReader().numDocs());
            List<String> categories = currentSource.getLeafCategories();
            res.put("categories_count", categories != null ? categories.size() : 0);

            return res.toJSONString();
        } catch (Exception e) {
            return errorJson("Failed to open case: " + e.getMessage());
        }
    }

    public synchronized String listCases() {
        JSONObject res = new JSONObject();
        if (currentSource == null) {
            res.put("opened", false);
            res.put("message", "No case currently opened. Use iped_open_case first.");
        } else {
            res.put("opened", true);
            res.put("case_dir", currentCaseDir.getAbsolutePath());
            res.put("total_items", currentSource.getTotalItems());
            res.put("num_docs", currentSource.getReader().numDocs());
        }
        return res.toJSONString();
    }

    @SuppressWarnings("unchecked")
    public synchronized String search(String query, int limit, int offset, List<String> fields) {
        if (currentSource == null) {
            return errorJson("No case opened. Call iped_open_case first.");
        }
        try {
            if (limit <= 0) limit = 50;
            if (limit > 200) limit = 200;
            if (offset < 0) offset = 0;

            String cleanQuery = query == null || query.trim().isEmpty() ? "*" : query.trim();
            cleanQuery = cleanQuery.replaceAll("/", "\\\\/");

            IPEDSearcher searcher = new IPEDSearcher(currentSource, cleanQuery);
            SearchResult result = searcher.search();

            int[] ids = result.getIds();
            int total = ids.length;

            JSONArray itemsArr = new JSONArray();
            int start = Math.min(offset, ids.length);
            int end = Math.min(start + limit, ids.length);

            List<String> defaultFields = Arrays.asList("id", "name", "path", "category", "length", "hash");
            List<String> selectedFields = (fields != null && !fields.isEmpty()) ? fields : defaultFields;

            for (int i = start; i < end; i++) {
                int id = ids[i];
                int luceneId = currentSource.getLuceneId(id);
                Document doc = currentSource.getReader().document(luceneId);

                JSONObject itemJson = new JSONObject();
                itemJson.put("id", id);

                for (String f : selectedFields) {
                    if ("id".equalsIgnoreCase(f)) continue;
                    String val = doc.get(f);
                    if (val != null) {
                        itemJson.put(f, val);
                    }
                }
                itemsArr.add(itemJson);
            }

            JSONObject res = new JSONObject();
            res.put("query", query);
            res.put("total_hits", total);
            res.put("offset", start);
            res.put("limit", limit);
            res.put("returned", itemsArr.size());
            res.put("items", itemsArr);

            return res.toJSONString();
        } catch (Exception e) {
            return errorJson("Search error: " + e.getMessage());
        }
    }

    @SuppressWarnings("unchecked")
    public synchronized String getItem(int id) {
        if (currentSource == null) {
            return errorJson("No case opened. Call iped_open_case first.");
        }
        try {
            if (id < 0 || id >= currentSource.getTotalItems()) {
                return errorJson("Invalid item id: " + id);
            }
            int luceneId = currentSource.getLuceneId(id);
            Document doc = currentSource.getReader().document(luceneId);

            JSONObject props = new JSONObject();
            for (IndexableField f : doc.getFields()) {
                String[] vals = doc.getValues(f.name());
                if (vals.length == 1) {
                    props.put(f.name(), vals[0]);
                } else if (vals.length > 1) {
                    JSONArray arr = new JSONArray();
                    Collections.addAll(arr, vals);
                    props.put(f.name(), arr);
                }
            }

            JSONObject res = new JSONObject();
            res.put("id", id);
            res.put("lucene_id", luceneId);
            res.put("properties", props);

            if (currentSource.getBookmarks() != null) {
                res.put("bookmarks", currentSource.getBookmarks().getBookmarkList(id));
                res.put("selected", currentSource.getBookmarks().isChecked(id));
            }

            return res.toJSONString();
        } catch (Exception e) {
            return errorJson("Get item error: " + e.getMessage());
        }
    }

    public synchronized String getText(int id, int offsetChars, int maxChars) {
        if (currentSource == null) {
            return errorJson("No case opened. Call iped_open_case first.");
        }
        try {
            if (id < 0 || id >= currentSource.getTotalItems()) {
                return errorJson("Invalid item id: " + id);
            }
            if (offsetChars < 0) offsetChars = 0;
            if (maxChars <= 0) maxChars = 4000;
            if (maxChars > 20000) maxChars = 20000;

            IItem item = currentSource.getItemByID(id);
            if (item == null) {
                return errorJson("Item not found: " + id);
            }

            StringWriter sw = new StringWriter();
            StandardParser parser = new StandardParser();
            ParseContext context = new ParseContext();
            Metadata metadata = new Metadata();
            ParsingTask.fillMetadata(item, metadata);
            parser.setPrintMetadata(false);

            ToTextContentHandler handler = new ToTextContentHandler(sw);
            try (TikaInputStream is = item.getTikaStream()) {
                parser.parse(is, handler, metadata, context);
            }

            String fullText = sw.toString();
            int totalLen = fullText.length();

            int start = Math.min(offsetChars, totalLen);
            int end = Math.min(start + maxChars, totalLen);
            String chunk = fullText.substring(start, end);

            JSONObject res = new JSONObject();
            res.put("id", id);
            res.put("total_chars", totalLen);
            res.put("offset_chars", start);
            res.put("returned_chars", chunk.length());
            res.put("has_more", end < totalLen);
            res.put("text", chunk);

            return res.toJSONString();
        } catch (Exception e) {
            return errorJson("Get text error: " + e.getMessage());
        }
    }

    @SuppressWarnings("unchecked")
    public synchronized String getCategories() {
        if (currentSource == null) {
            return errorJson("No case opened. Call iped_open_case first.");
        }
        try {
            List<String> leaves = currentSource.getLeafCategories();
            JSONArray arr = new JSONArray();
            if (leaves != null) {
                for (String cat : leaves) {
                    JSONObject c = new JSONObject();
                    c.put("category", cat);
                    arr.add(c);
                }
            }
            JSONObject res = new JSONObject();
            res.put("categories", arr);
            return res.toJSONString();
        } catch (Exception e) {
            return errorJson("Categories error: " + e.getMessage());
        }
    }

    @SuppressWarnings("unchecked")
    public synchronized String listBookmarks() {
        if (currentSource == null) {
            return errorJson("No case opened. Call iped_open_case first.");
        }
        try {
            JSONObject res = new JSONObject();
            JSONArray arr = new JSONArray();
            if (currentSource.getBookmarks() != null) {
                Map<Integer, String> map = currentSource.getBookmarks().getBookmarkMap();
                if (map != null) {
                    for (Map.Entry<Integer, String> entry : map.entrySet()) {
                        JSONObject bobj = new JSONObject();
                        bobj.put("id", entry.getKey());
                        bobj.put("name", entry.getValue());
                        bobj.put("count", currentSource.getBookmarks().getBookmarkCount(entry.getKey()));
                        arr.add(bobj);
                    }
                }
            }
            res.put("bookmarks", arr);
            return res.toJSONString();
        } catch (Exception e) {
            return errorJson("Bookmarks error: " + e.getMessage());
        }
    }

    public synchronized String addBookmark(int id, String bookmark) {
        if (currentSource == null) {
            return errorJson("No case opened. Call iped_open_case first.");
        }
        try {
            if (currentSource.getBookmarks() == null) {
                return errorJson("Bookmarks not available in this case.");
            }
            if (id < 0 || id >= currentSource.getTotalItems()) {
                return errorJson("Invalid item id: " + id);
            }
            int bookmarkId = currentSource.getBookmarks().getBookmarkId(bookmark);
            if (bookmarkId == -1) {
                bookmarkId = currentSource.getBookmarks().newBookmark(bookmark);
            }
            currentSource.getBookmarks().addBookmark(Collections.singletonList(id), bookmarkId);
            currentSource.getBookmarks().saveState();

            JSONObject res = new JSONObject();
            res.put("status", "ok");
            res.put("id", id);
            res.put("bookmark", bookmark);
            res.put("bookmark_id", bookmarkId);
            return res.toJSONString();
        } catch (Exception e) {
            return errorJson("Add bookmark error: " + e.getMessage());
        }
    }

    public synchronized String startProcess(String evidence, String output, String profile, String extraArgs) {
        try {
            if (activeProcess != null && activeProcess.isAlive()) {
                return errorJson("A process job is already running (PID: " + activeProcess.pid() + ")");
            }

            File outDir = new File(output);
            if (outDir.exists() && outDir.list() != null && outDir.list().length > 0) {
                return errorJson("Output directory already exists and is not empty: " + output);
            }

            File evFile = new File(evidence);
            if (!evFile.exists()) {
                return errorJson("Evidence file or directory does not exist: " + evidence);
            }

            File baseDir = new File(System.getProperty("user.dir", "."));
            File ipedJar = new File(baseDir, "target/release/iped-4.4.0-SNAPSHOT/iped.jar");
            if (!ipedJar.exists()) {
                ipedJar = new File("D:\\iped-bench\\rel-lite\\iped.jar");
            }

            List<String> cmd = new ArrayList<>();
            cmd.add("java");
            cmd.add("-jar");
            cmd.add(ipedJar.getAbsolutePath());
            cmd.add("-d");
            cmd.add(evidence);
            cmd.add("-o");
            cmd.add(output);
            cmd.add("--nogui");
            if (profile != null && !profile.isEmpty() && !profile.equals("default")) {
                cmd.add("-profile");
                cmd.add(profile);
            }
            if (extraArgs != null && !extraArgs.trim().isEmpty()) {
                String[] parts = extraArgs.trim().split("\\s+");
                Collections.addAll(cmd, parts);
            }

            activeStdoutLog = new File(output + ".stdout.log");
            activeStderrLog = new File(output + ".stderr.log");
            activeEvidencePath = evidence;
            activeOutputPath = output;
            activeProcessStartMs = System.currentTimeMillis();

            ProcessBuilder pb = new ProcessBuilder(cmd);
            pb.redirectOutput(activeStdoutLog);
            pb.redirectError(activeStderrLog);
            activeProcess = pb.start();

            JSONObject res = new JSONObject();
            res.put("status", "started");
            res.put("pid", activeProcess.pid());
            res.put("evidence", evidence);
            res.put("output", output);
            res.put("profile", profile != null ? profile : "lite");
            res.put("stdout_log", activeStdoutLog.getAbsolutePath());
            return res.toJSONString();
        } catch (Exception e) {
            return errorJson("Failed to start process: " + e.getMessage());
        }
    }

    public synchronized String processStatus() {
        JSONObject res = new JSONObject();
        if (activeProcess == null) {
            res.put("status", "idle");
            res.put("message", "No process job has been launched in this session.");
            return res.toJSONString();
        }

        boolean alive = activeProcess.isAlive();
        long elapsedSec = (System.currentTimeMillis() - activeProcessStartMs) / 1000;
        res.put("pid", activeProcess.pid());
        res.put("alive", alive);
        res.put("elapsed_seconds", elapsedSec);
        res.put("evidence", activeEvidencePath);
        res.put("output", activeOutputPath);

        if (!alive) {
            res.put("exit_code", activeProcess.exitValue());
        }

        List<String> lastLines = new ArrayList<>();
        if (activeStdoutLog != null && activeStdoutLog.exists()) {
            try (BufferedReader br = new BufferedReader(new InputStreamReader(new FileInputStream(activeStdoutLog), StandardCharsets.UTF_8))) {
                String line;
                while ((line = br.readLine()) != null) {
                    if (lastLines.size() >= 10) {
                        lastLines.remove(0);
                    }
                    lastLines.add(line);
                }
            } catch (Exception ignored) {
            }
        }
        JSONArray linesArr = new JSONArray();
        linesArr.addAll(lastLines);
        res.put("recent_stdout", linesArr);

        return res.toJSONString();
    }

    @SuppressWarnings("unchecked")
    public static JSONArray getToolsList() {
        JSONArray tools = new JSONArray();

        tools.add(tool("iped_open_case", "Opens an already processed IPED case directory for searching, reading text, and analysis.",
                param("path", "string", "Absolute path to the IPED case directory (contains 'index' or 'iped' folder)", true)));

        tools.add(tool("iped_list_cases", "Returns information about the currently opened IPED case (total items, document count).",
                new JSONObject()));

        JSONObject searchProps = new JSONObject();
        searchProps.put("query", schemaProp("string", "Lucene search query (e.g. '*' or 'name:*.pdf' or 'hash:xyz' or 'categoria:Email')"));
        searchProps.put("limit", schemaProp("integer", "Maximum number of items to return (default 50, max 200)"));
        searchProps.put("offset", schemaProp("integer", "Offset for pagination (default 0)"));
        JSONObject searchSchema = objSchema(searchProps, Collections.singletonList("query"));
        tools.add(tool("iped_search", "Searches the opened IPED case using Lucene query syntax. Returns compact, paginated items.", searchSchema));

        tools.add(tool("iped_get_item", "Retrieves complete metadata properties and attributes for an item by its ID.",
                param("id", "integer", "The document ID to retrieve", true)));

        JSONObject textProps = new JSONObject();
        textProps.put("id", schemaProp("integer", "The document ID"));
        textProps.put("offset_chars", schemaProp("integer", "Offset character index for pagination (default 0)"));
        textProps.put("max_chars", schemaProp("integer", "Maximum characters to return (default 4000, max 20000)"));
        JSONObject textSchema = objSchema(textProps, Collections.singletonList("id"));
        tools.add(tool("iped_get_text", "Retrieves extracted text for a specific document ID with character pagination.", textSchema));

        tools.add(tool("iped_categories", "Lists all available categories and subcategories in the case.", new JSONObject()));

        tools.add(tool("iped_bookmarks_list", "Lists all bookmarks and their document counts in the opened case.", new JSONObject()));

        JSONObject bmProps = new JSONObject();
        bmProps.put("id", schemaProp("integer", "The document ID to bookmark"));
        bmProps.put("bookmark", schemaProp("string", "The name of the bookmark category"));
        tools.add(tool("iped_bookmarks_add", "Adds a bookmark to a specific document ID.", objSchema(bmProps, Arrays.asList("id", "bookmark"))));

        JSONObject procProps = new JSONObject();
        procProps.put("evidence", schemaProp("string", "Absolute path to evidence file or folder (E01, UFDR, mbox, disk, dir)"));
        procProps.put("output", schemaProp("string", "Absolute path to output folder (must be empty or not exist)"));
        procProps.put("profile", schemaProp("string", "Processing profile (default: 'lite')"));
        procProps.put("extra_args", schemaProp("string", "Extra JVM or IPED arguments (e.g. '-Xmx2g')"));
        tools.add(tool("iped_process", "Launches an IPED processing job in background and tracks its progress.", objSchema(procProps, Arrays.asList("evidence", "output"))));

        tools.add(tool("iped_process_status", "Checks the status, running time, and recent output of any active or finished processing job.", new JSONObject()));

        return tools;
    }

    @SuppressWarnings("unchecked")
    private static JSONObject tool(String name, String description, JSONObject schema) {
        JSONObject t = new JSONObject();
        t.put("name", name);
        t.put("description", description);
        t.put("inputSchema", schema.isEmpty() ? objSchema(new JSONObject(), Collections.emptyList()) : schema);
        return t;
    }

    @SuppressWarnings("unchecked")
    private static JSONObject param(String name, String type, String desc, boolean required) {
        JSONObject props = new JSONObject();
        props.put(name, schemaProp(type, desc));
        return objSchema(props, required ? Collections.singletonList(name) : Collections.emptyList());
    }

    @SuppressWarnings("unchecked")
    private static JSONObject schemaProp(String type, String desc) {
        JSONObject p = new JSONObject();
        p.put("type", type);
        p.put("description", desc);
        return p;
    }

    @SuppressWarnings("unchecked")
    private static JSONObject objSchema(JSONObject properties, List<String> required) {
        JSONObject s = new JSONObject();
        s.put("type", "object");
        s.put("properties", properties);
        if (required != null && !required.isEmpty()) {
            JSONArray reqArr = new JSONArray();
            reqArr.addAll(required);
            s.put("required", reqArr);
        }
        return s;
    }

    @SuppressWarnings("unchecked")
    private static String errorJson(String msg) {
        JSONObject err = new JSONObject();
        err.put("error", msg);
        return err.toJSONString();
    }
}

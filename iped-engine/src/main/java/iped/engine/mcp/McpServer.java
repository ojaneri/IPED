package iped.engine.mcp;

import java.io.BufferedReader;
import java.io.InputStreamReader;
import java.io.OutputStreamWriter;
import java.io.PrintStream;
import java.io.PrintWriter;
import java.nio.charset.StandardCharsets;
import java.util.List;

import org.json.simple.JSONArray;
import org.json.simple.JSONObject;
import org.json.simple.parser.JSONParser;

public class McpServer {

    private final McpToolHandler handler;
    private final PrintWriter out;
    private final JSONParser parser;

    public McpServer(PrintStream realOut) {
        this.handler = new McpToolHandler();
        this.out = new PrintWriter(new OutputStreamWriter(realOut, StandardCharsets.UTF_8), true);
        this.parser = new JSONParser();
    }

    public McpServer() {
        this(System.out);
    }

    public void run() {
        System.err.println("[iped-mcp] Server started on stdio");
        try (BufferedReader reader = new BufferedReader(new InputStreamReader(System.in, StandardCharsets.UTF_8))) {
            String line;
            while ((line = reader.readLine()) != null) {
                line = line.trim();
                if (line.isEmpty()) {
                    continue;
                }
                // Handle optional Content-Length headers if sent by some client transports
                if (line.startsWith("Content-Length:")) {
                    continue;
                }

                try {
                    Object parsed = parser.parse(line);
                    if (parsed instanceof JSONObject) {
                        handleMessage((JSONObject) parsed);
                    }
                } catch (Exception e) {
                    System.err.println("[iped-mcp] Error parsing JSON message: " + e.getMessage() + " Line: " + line);
                }
            }
        } catch (Exception e) {
            System.err.println("[iped-mcp] Stdio reader exception: " + e.getMessage());
        }
        System.err.println("[iped-mcp] Server stopped");
    }

    @SuppressWarnings("unchecked")
    private void handleMessage(JSONObject msg) {
        Object id = msg.get("id");
        String method = (String) msg.get("method");
        JSONObject params = (JSONObject) msg.get("params");
        if (params == null) {
            params = new JSONObject();
        }

        if (method == null) {
            return;
        }

        // Notification (no id)
        if (id == null) {
            if ("notifications/initialized".equals(method)) {
                System.err.println("[iped-mcp] Client initialized notification received");
            }
            return;
        }

        JSONObject response = new JSONObject();
        response.put("jsonrpc", "2.0");
        response.put("id", id);

        switch (method) {
            case "initialize": {
                JSONObject result = new JSONObject();
                result.put("protocolVersion", "2024-11-05");

                JSONObject caps = new JSONObject();
                caps.put("tools", new JSONObject());
                result.put("capabilities", caps);

                JSONObject serverInfo = new JSONObject();
                serverInfo.put("name", "iped-mcp");
                serverInfo.put("version", "4.4.0-snapshot");
                result.put("serverInfo", serverInfo);

                response.put("result", result);
                send(response);
                break;
            }
            case "ping": {
                response.put("result", new JSONObject());
                send(response);
                break;
            }
            case "tools/list": {
                JSONObject result = new JSONObject();
                result.put("tools", McpToolHandler.getToolsList());
                response.put("result", result);
                send(response);
                break;
            }
            case "tools/call": {
                String toolName = (String) params.get("name");
                JSONObject args = (JSONObject) params.get("arguments");
                if (args == null) {
                    args = new JSONObject();
                }

                String textResult;
                boolean isError = false;
                try {
                    textResult = executeTool(toolName, args);
                    if (textResult.contains("\"error\":")) {
                        isError = true;
                    }
                } catch (Exception e) {
                    textResult = "{\"error\": \"" + e.getMessage() + "\"}";
                    isError = true;
                }

                JSONObject result = new JSONObject();
                JSONArray contentArr = new JSONArray();
                JSONObject textItem = new JSONObject();
                textItem.put("type", "text");
                textItem.put("text", textResult);
                contentArr.add(textItem);

                result.put("content", contentArr);
                result.put("isError", isError);

                response.put("result", result);
                send(response);
                break;
            }
            default: {
                JSONObject err = new JSONObject();
                err.put("code", -32601);
                err.put("message", "Method not found: " + method);
                response.put("error", err);
                send(response);
                break;
            }
        }
    }

    @SuppressWarnings("unchecked")
    private String executeTool(String name, JSONObject args) {
        if ("iped_open_case".equals(name)) {
            String path = (String) args.get("path");
            return handler.openCase(path);
        } else if ("iped_list_cases".equals(name)) {
            return handler.listCases();
        } else if ("iped_search".equals(name)) {
            String query = (String) args.get("query");
            Number limitNum = (Number) args.get("limit");
            Number offsetNum = (Number) args.get("offset");
            int limit = limitNum != null ? limitNum.intValue() : 50;
            int offset = offsetNum != null ? offsetNum.intValue() : 0;
            List<String> fields = (List<String>) args.get("fields");
            return handler.search(query, limit, offset, fields);
        } else if ("iped_get_item".equals(name)) {
            Number idNum = (Number) args.get("id");
            if (idNum == null) {
                return "{\"error\": \"Missing required argument: id\"}";
            }
            return handler.getItem(idNum.intValue());
        } else if ("iped_get_text".equals(name)) {
            Number idNum = (Number) args.get("id");
            if (idNum == null) {
                return "{\"error\": \"Missing required argument: id\"}";
            }
            Number offsetNum = (Number) args.get("offset_chars");
            Number maxNum = (Number) args.get("max_chars");
            int offset = offsetNum != null ? offsetNum.intValue() : 0;
            int maxChars = maxNum != null ? maxNum.intValue() : 4000;
            return handler.getText(idNum.intValue(), offset, maxChars);
        } else if ("iped_categories".equals(name)) {
            return handler.getCategories();
        } else if ("iped_bookmarks_list".equals(name)) {
            return handler.listBookmarks();
        } else if ("iped_bookmarks_add".equals(name)) {
            Number idNum = (Number) args.get("id");
            String bookmark = (String) args.get("bookmark");
            if (idNum == null || bookmark == null) {
                return "{\"error\": \"Missing required arguments: id, bookmark\"}";
            }
            return handler.addBookmark(idNum.intValue(), bookmark);
        } else if ("iped_process".equals(name)) {
            String evidence = (String) args.get("evidence");
            String output = (String) args.get("output");
            String profile = (String) args.get("profile");
            String extraArgs = (String) args.get("extra_args");
            if (evidence == null || output == null) {
                return "{\"error\": \"Missing required arguments: evidence, output\"}";
            }
            return handler.startProcess(evidence, output, profile, extraArgs);
        } else if ("iped_process_status".equals(name)) {
            return handler.processStatus();
        } else {
            return "{\"error\": \"Unknown tool: " + name + "\"}";
        }
    }

    private void send(JSONObject obj) {
        String json = obj.toJSONString();
        out.println(json);
        out.flush();
    }

    public static void main(String[] args) {
        PrintStream realOut = System.out;
        System.setOut(System.err); // redirect all library output to stderr!

        McpServer server = new McpServer(realOut);
        if (args.length >= 2 && "--open-case".equals(args[0])) {
            System.err.println("[iped-mcp] Pre-opening case: " + args[1]);
            server.handler.openCase(args[1]);
        }
        server.run();
    }
}

package iped.engine.mcp;

import static org.junit.Assert.assertEquals;
import static org.junit.Assert.assertFalse;
import static org.junit.Assert.assertNotNull;
import static org.junit.Assert.assertTrue;

import java.io.ByteArrayInputStream;
import java.io.ByteArrayOutputStream;
import java.io.PrintStream;
import java.nio.charset.StandardCharsets;

import org.json.simple.JSONArray;
import org.json.simple.JSONObject;
import org.json.simple.parser.JSONParser;
import org.junit.Test;

public class McpServerTest {

    @Test
    public void testGetToolsList() {
        JSONArray tools = McpToolHandler.getToolsList();
        assertNotNull(tools);
        assertEquals(10, tools.size());

        boolean hasSearch = false;
        boolean hasOpen = false;
        boolean hasProcess = false;
        for (Object obj : tools) {
            JSONObject t = (JSONObject) obj;
            String name = (String) t.get("name");
            if ("iped_search".equals(name)) hasSearch = true;
            if ("iped_open_case".equals(name)) hasOpen = true;
            if ("iped_process".equals(name)) hasProcess = true;
        }
        assertTrue(hasSearch);
        assertTrue(hasOpen);
        assertTrue(hasProcess);
    }

    @Test
    public void testServerProtocolFlow() throws Exception {
        String input = "{\"jsonrpc\":\"2.0\",\"id\":1,\"method\":\"initialize\",\"params\":{}}\n"
                + "{\"jsonrpc\":\"2.0\",\"method\":\"notifications/initialized\"}\n"
                + "{\"jsonrpc\":\"2.0\",\"id\":2,\"method\":\"ping\",\"params\":{}}\n"
                + "{\"jsonrpc\":\"2.0\",\"id\":3,\"method\":\"tools/list\",\"params\":{}}\n"
                + "{\"jsonrpc\":\"2.0\",\"id\":4,\"method\":\"tools/call\",\"params\":{\"name\":\"iped_list_cases\",\"arguments\":{}}}\n"
                + "{\"jsonrpc\":\"2.0\",\"id\":5,\"method\":\"tools/call\",\"params\":{\"name\":\"unknown_tool\",\"arguments\":{}}}\n";

        ByteArrayInputStream inStream = new ByteArrayInputStream(input.getBytes(StandardCharsets.UTF_8));
        ByteArrayOutputStream outStream = new ByteArrayOutputStream();

        PrintStream origOut = System.out;
        java.io.InputStream origIn = System.in;
        try {
            System.setIn(inStream);
            System.setOut(new PrintStream(outStream, true, StandardCharsets.UTF_8));

            McpServer server = new McpServer();
            server.run();

        } finally {
            System.setOut(origOut);
            System.setIn(origIn);
        }

        String outputStr = outStream.toString(StandardCharsets.UTF_8);
        String[] lines = outputStr.split("\\r?\\n");
        assertEquals(5, lines.length);

        JSONParser parser = new JSONParser();

        // 1. initialize
        JSONObject initResp = (JSONObject) parser.parse(lines[0]);
        assertEquals(1L, initResp.get("id"));
        JSONObject initRes = (JSONObject) initResp.get("result");
        assertEquals("2024-11-05", initRes.get("protocolVersion"));
        JSONObject serverInfo = (JSONObject) initRes.get("serverInfo");
        assertEquals("iped-mcp", serverInfo.get("name"));

        // 2. ping
        JSONObject pingResp = (JSONObject) parser.parse(lines[1]);
        assertEquals(2L, pingResp.get("id"));
        assertNotNull(pingResp.get("result"));

        // 3. tools/list
        JSONObject listResp = (JSONObject) parser.parse(lines[2]);
        assertEquals(3L, listResp.get("id"));
        JSONObject listRes = (JSONObject) listResp.get("result");
        JSONArray tools = (JSONArray) listRes.get("tools");
        assertEquals(10, tools.size());

        // 4. tools/call iped_list_cases
        JSONObject callResp = (JSONObject) parser.parse(lines[3]);
        assertEquals(4L, callResp.get("id"));
        JSONObject callRes = (JSONObject) callResp.get("result");
        assertFalse((Boolean) callRes.get("isError"));
        JSONArray content = (JSONArray) callRes.get("content");
        JSONObject textItem = (JSONObject) content.get(0);
        assertTrue(((String) textItem.get("text")).contains("No case currently opened"));

        // 5. tools/call unknown_tool
        JSONObject errResp = (JSONObject) parser.parse(lines[4]);
        assertEquals(5L, errResp.get("id"));
        JSONObject errRes = (JSONObject) errResp.get("result");
        assertTrue((Boolean) errRes.get("isError"));
    }
}

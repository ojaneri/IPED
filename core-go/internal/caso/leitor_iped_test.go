package caso

import (
	"os"
	"path/filepath"
	"testing"
)

func TestImportarCasoIPED(t *testing.T) {
	tmpDir := t.TempDir()

	// 1. Cria diretório simulando saída do IPED com FileList.csv
	dirIPED := filepath.Join(tmpDir, "saida-iped")
	os.MkdirAll(dirIPED, 0755)

	conteudoCSV := `"Name","Link","Size","Extension","Bookmark","Category","MD5","SHA1","Deleted","Carved","Accessed","Modified","Created","Path","TrackId"
"documento.docx","","15200","docx","Relevante","Documentos","E10ADC3949BA59ABBE56E057F20F883E","C1296D4685E604F1AD28DECB894F48A6F0FE67A4","false","false","","2026-10-30T10:00:00Z","","pasta/documento.docx","track_101"
"apagado.jpg","","84200","jpg","","Imagens","F10ADC3949BA59ABBE56E057F20F883F","D1296D4685E604F1AD28DECB894F48A6F0FE67A5","true","false","","2026-10-30T11:00:00Z","","fotos/apagado.jpg","track_102"
"recuperado.pdf","","42000","pdf","","Documentos","A10ADC3949BA59ABBE56E057F20F883A","B1296D4685E604F1AD28DECB894F48A6F0FE67A6","false","true","","2026-10-30T12:00:00Z","","carved/recuperado.pdf","track_103"
`
	os.WriteFile(filepath.Join(dirIPED, "FileList.csv"), []byte(conteudoCSV), 0644)

	// 2. Cria caso no SQLite
	pastaCaso := filepath.Join(tmpDir, "caso-db")
	c, err := Novo(pastaCaso, "Caso Importação IPED", "Perito")
	if err != nil {
		t.Fatalf("falha ao criar caso: %v", err)
	}
	defer c.Fechar()

	// 3. Importa o caso IPED
	res, err := c.ImportarCasoIPED(dirIPED, 1)
	if err != nil {
		t.Fatalf("falha ao importar caso IPED: %v", err)
	}

	if res.TotalItens != 3 {
		t.Fatalf("esperava 3 itens importados, obteve %d", res.TotalItens)
	}
	if res.Marcacoes != 1 {
		t.Fatalf("esperava 1 marcação importada, obteve %d", res.Marcacoes)
	}
	if res.Apagados != 1 {
		t.Fatalf("esperava 1 item apagado, obteve %d", res.Apagados)
	}
	if res.Recuperados != 1 {
		t.Fatalf("esperava 1 item recuperado (carved), obteve %d", res.Recuperados)
	}

	// 4. Confere busca de eventos
	eventos, total, err := c.BuscarEventos("documento.docx", 10, 0)
	if err != nil || total != 1 {
		t.Fatalf("falha na busca de documento importado: total=%d, err=%v", total, err)
	}
	if len(eventos) != 1 || eventos[0].OrigemLocal != "track_101" {
		t.Fatalf("evento recuperado inconsistente: %+v", eventos[0])
	}

	// 5. Confere lista de marcações (bookmarks)
	bks, err := c.ListarMarcacoes()
	if err != nil || len(bks) != 1 {
		t.Fatalf("esperava 1 bookmark, obteve %d", len(bks))
	}
	if bks[0].Nome != "Relevante" {
		t.Fatalf("nome do bookmark divergente: %s", bks[0].Nome)
	}
}

#### Instalación y uso

```sh
go get github.com/zerobounce/zerobouncego
```

Este paquete utiliza la API de ZeroBounce, la cual requiere una clave de API. Esta clave se puede proporcionar de tres formas diferentes:

1. A través de una variable de entorno `ZERO_BOUNCE_API_KEY` (cargada automáticamente en el código).
2. A través de un archivo .env que contiene `ZERO_BOUNCE_API_KEY` y luego llamando al siguiente método antes de su uso:
   ```go
   zerobouncego.ImportApiKeyFromEnvFile()
   ```
3. Estableciéndola explícitamente en el código utilizando el siguiente método:
   ```go
   zerobouncego.SetApiKey("mysecretapikey")
   ```

#### Métodos genéricos de la API

```go
package main

import (
	"fmt"
	"time"

	"github.com/zerobounce/zerobouncego"
)

func main() {
	zerobouncego.SetApiKey("... Tu clave de API ...")

	// Verifica los créditos de tu cuenta
	credits, error_ := zerobouncego.GetCredits()
	if error_ != nil {
		fmt.Println("Error en la obtención de los créditos: ", error_.Error())
	} else {
		fmt.Println("Créditos restantes:", credits.Credits())
	}

	// Verifica el uso de la API en tu cuenta
	start_time := time.Date(2023, 1, 1, 0, 0, 0, 0, time.Local)
	end_time := time.Now()
	usage, error_ := zerobouncego.GetApiUsage(start_time, end_time)
	if error_ != nil {
		fmt.Println("Error en el uso de la API: ", error_.Error())
	} else {
		fmt.Println("Total de llamadas a la API: ", usage.Total)
	}
}
```

#### Validación

###### 1. Validación de un solo correo electrónico

```go
package main

import (
	"fmt"
	"os"
	"github.com/zerobounce/zerobouncego"
)

func main() {

	zerobouncego.APIKey = "... Tu clave de API ..."

	// Para consultar un solo correo electrónico y una IP
	// La IP también puede ser una cadena vacía
	response, error_ := zerobouncego.Validate("possible_typo@example.com", "123.123.123.123")

	if error_ != nil {
		fmt.Println("Se produjo un error: ", error_.Error())
	} else {
		// Ahora puedes verificar el estado
		if response.Status == zerobouncego.S_INVALID {
			fmt.Println("Este correo electrónico es válido")
		}

		// ... o el subestado
		if response.SubStatus == zerobouncego.SS_POSSIBLE_TYPO {
			fmt.Println("Este correo electrónico podría tener un error tipográfico")
		}
	}
}
```

###### 2. Validación en lote

```go
package main

import (
	"fmt"

	"github.com/zerobounce/zerobouncego"
)

func main() {
	zerobouncego.SetApiKey("... Tu clave de API ...")

	emails_to_validate := []zerobounce

go.EmailToValidate{
		{EmailAddress: "disposable@example.com", IPAddress: "99.110.204.1"},
		{EmailAddress: "invalid@example.com", IPAddress: "1.1.1.1"},
		{EmailAddress: "valid@example.com"},
		{EmailAddress: "toxic@example.com"},
	}

	response, error_ := zerobouncego.ValidateBatch(emails_to_validate)
	if error_ != nil {
		fmt.Println("Se produjo un error al validar en lotes: ", error_.Error())
	} else {
		fmt.Printf("Se obtuvieron %d resultados exitosos y %d resultados de error\n", len(response.EmailBatch), len(response.Errors))
		if len(response.EmailBatch) > 0 {
			fmt.Printf(
				"El correo electrónico '%s' tiene un estado '%s' y un subestado '%s'\n",
				response.EmailBatch[0].Address,
				response.EmailBatch[0].Status,
				response.EmailBatch[0].SubStatus,
			)
		}
	}
}
```

###### 3. Validación de archivos a granel

```go
package main

import (
	"fmt"
	"os"
	"time"

	"github.com/zerobounce/zerobouncego"
)


func main() {
	zerobouncego.SetApiKey("... Tu clave de API ...")
	import_file_path := "RUTA_AL_ARCHIVO_CSV_A_IMPORTAR"
	result_file_path := "RUTA_AL_ARCHIVO_CSV_A_EXPORTAR"

	file, error_ := os.Open(import_file_path)
	if error_ != nil {
		fmt.Println("Error al abrir el archivo: ", error_.Error())
		return
	}

	defer file.Close()
	csv_file := zerobouncego.CsvFile{
		File: file, HasHeaderRow: false, EmailAddressColumn: 1, FileName: "emails.csv",
	}
	submit_response, error_ := zerobouncego.BulkValidationSubmit(csv_file, false)
	if error_ != nil {
		fmt.Println("Error al enviar los datos: ", error_.Error())
		return
	}

	fmt.Println("ID de archivo enviado: ", submit_response.FileId)
	var file_status *zerobouncego.FileStatusResponse
	file_status, _ = zerobouncego.BulkValidationFileStatus(submit_response.FileId)
	fmt.Println("Estado del archivo: ", file_status.FileStatus)
	fmt.Println("Porcentaje de finalización: ", file_status.Percentage(), "%")

	// Espera a que el archivo se complete
	fmt.Println()
	fmt.Println("Esperando a que el archivo se complete")
	var seconds_waited int = 1
	for file_status.Percentage() != 100. {
		time.Sleep(time.Duration(seconds_waited) * time.Second)
		if seconds_waited < 10 {
			seconds_waited += 1
		}

		file_status, error_ = zerobouncego.BulkValidationFileStatus(submit_response.FileId)
		if error_ != nil {
			fmt.Print()
			fmt.Print("Error al obtener el estado: ", error_.Error())
			return
		}
		fmt.Printf("..%.2f%% ", file_status.Percentage())
	}
	fmt.Println()
	fmt.Println("Validación de archivos completada")

	// Guardar el resultado de la validación
	result_file, error_ := os.OpenFile(result_file_path, os.O_RDWR | os.O_CREATE, 0644)
	if error_ != nil {
		fmt.Println("Error al crear el archivo de resultado: ", error_.Error())
		return
	}
	error_ = zerobouncego.BulkValidationResult(submit_response.FileId, result_file)


	defer result_file.Close()
	if error_ != nil {
		fmt.Println("Error al obtener el resultado de la validación: ", error_.Error())
		return
	}
	fmt.Printf("Resultado de validación guardado en la ruta: %s\n", result_file_path)

	// Eliminar el archivo de resultado después de guardarlo
	delete_status, error_ := zerobouncego.BulkValidationFileDelete(file_status.FileId)
	if error_ != nil {
		fmt.Println("Error al eliminar el archivo: ", error_.Error())
		return
	}
	fmt.Println(delete_status)
}

```

Ejemplo de archivo de importación (CSV):
```csv
disposable@example.com
invalid@example.com
valid@example.com
toxic@example.com

```

Ejemplo de archivo de exportación (CSV):
```csv
"Email Address","ZB Status","ZB Sub Status","ZB Account","ZB Domain","ZB First Name","ZB Last Name","ZB Gender","ZB Free Email","ZB MX Found","ZB MX Record","ZB SMTP Provider","ZB Did You Mean"
"disposable@example.com","do_not_mail","disposable","","","zero","bounce","male","False","true","mx.example.com","example",""
"invalid@example.com","invalid","mailbox_not_found","","","zero","bounce","male","False","true","mx.example.com","example",""
"valid@example.com","valid","","","","zero","bounce","male","False","true","mx.example.com","example",""
"toxic@example.com","do_not_mail","toxic","","","zero","bounce","male","False","true","mx.example.com","example",""
"mailbox_not_found@example.com","invalid","mailbox_not_found","","","zero","bounce","male","False","true","mx.example.com","example",""
"failed_syntax_check@example.com","invalid","failed_syntax_check","","","zero","bounce","male","False","true","mx.example.com","example",""

```


###### 4. Puntuación de IA

```go
package main

import (
	"fmt"
	"os"
	"time"

	"github.com/zerobounce/zerobouncego"
)


func main() {
	zerobouncego.SetApiKey("... Tu clave de API ...")
	zerobouncego.ImportApiKeyFromEnvFile()
	import_file_path := "./emails.csv"
	result_file_path := "./validation_result.csv"

	file, error_ := os.Open(import_file_path)
	if error_ != nil {
		fmt.Println("Error al abrir el archivo: ", error_.Error())
		return
	}

	defer file.Close()
	csv_file := zerobouncego.CsvFile{
		File: file, HasHeaderRow: false, EmailAddressColumn: 1, FileName: "emails.csv",
	}
	submit_response, error_ := zerobouncego.AiScoringFileSubmit(csv_file, false)
	if error_ != nil {
		fmt.Println("Error al enviar los datos: ", error_.Error())
		return
	}

	fmt.Println("ID de archivo enviado: ", submit_response.FileId)
	var file_status *zerobouncego.FileStatusResponse
	file_status, _ = zerobouncego.AiScoringFileStatus(submit_response.FileId)
	fmt.Println("Estado del archivo: ", file_status.FileStatus)
	fmt.Println("Porcentaje de finalización: ", file_status.Percentage(), "%")

	// Espera a que el archivo se complete
	fmt.Println()
	fmt.Println("Esperando a que el archivo se complete")
	var seconds_waited int = 1
	for file_status.Percentage() != 100. {
		time.Sleep(time.Duration(seconds_waited) * time.Second)
		if seconds_wait

ed < 10 {
			seconds_waited += 1
		}

		file_status, error_ = zerobouncego.AiScoringFileStatus(submit_response.FileId)
		if error_ != nil {
			fmt.Print()
			fmt.Print("Error al obtener el estado: ", error_.Error())
			return
		}
		fmt.Printf("..%.2f%% ", file_status.Percentage())
	}
	fmt.Println()
	fmt.Println("Validación de archivos completada")

	// Guardar el resultado de la validación
	result_file, error_ := os.OpenFile(result_file_path, os.O_RDWR | os.O_CREATE, 0644)
	if error_ != nil {
		fmt.Println("Error al crear el archivo de resultado: ", error_.Error())
		return
	}
	error_ = zerobouncego.AiScoringResult(submit_response.FileId, result_file)
	defer result_file.Close()
	if error_ != nil {
		fmt.Println("Error al obtener el resultado de la validación: ", error_.Error())
		return
	}
	fmt.Printf("Resultado de validación guardado en la ruta: %s\n", result_file_path)

	// Eliminar el archivo de resultado después de guardarlo
	delete_status, error_ := zerobouncego.AiScoringFileDelete(file_status.FileId)
	if error_ != nil {
		fmt.Println("Error al eliminar el archivo: ", error_.Error())
		return
	}
	fmt.Println(delete_status)
}

```


Ejemplo de archivo de importación (CSV):
```csv
disposable@example.com
invalid@example.com
valid@example.com
toxic@example.com

```

Ejemplo de archivo de exportación (CSV):
```csv
"Email Address","ZeroBounceQualityScore"
"disposable@example.com","0"
"invalid@example.com","10"
"valid@example.com","10"
"toxic@example.com","2"

```

#### Pruebas

Este paquete contiene tanto pruebas unitarias como pruebas de integración (que están excluidas del conjunto de pruebas). Los archivos de prueba unitaria tienen un sufijo "_test.go" (como requiere Go) y las pruebas de integración tienen un sufijo ("_integration_t.go").

Para ejecutar las pruebas de integración:
- Establece la variable de entorno `ZERO_BOUNCE_API_KEY` con la clave de API adecuada.
- Renombra todos los archivos "_integration_t.go" a "_integration_test.go".
- Ejecuta las pruebas individuales o todas (`go test .`)

NOTA: Actualmente, las pruebas unitarias se pueden actualizar para que, eliminando la simulación y la configuración explícita de la clave de API, también funcionen como pruebas de integración SIEMPRE QUE se proporcione una clave de API válida a través del entorno.

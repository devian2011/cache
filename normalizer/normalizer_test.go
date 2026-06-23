package normalizer

import (
	"net/url"
	"testing"
)

func TestFastNormalizeQuery(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    string
		wantErr bool
	}{
		{
			name:  "Базовый случай: сортировка значений и индексов",
			input: "filters[key]=termal&filters[key]=awasp&filter[a]=2&filter[a]=1",
			want:  "filter[a]=1&filter[a]=2&filters[key]=awasp&filters[key]=termal",
		},
		{
			name:  "Ваш пример: плоские ключи, сортировка и ассоциативные массивы",
			input: "sort=price_asc&filters[key]&position[key]",
			want:  "filters[key]=&position[key]=&sort=price_asc",
		},
		{
			name:  "Смешанный сложный запрос со значениями и без",
			input: "sort=price_asc&filters[key]=termal&position[key]=top&page=2&debug",
			want:  "debug=&filters[key]=termal&page=2&position[key]=top&sort=price_asc",
		},
		{
			name:  "Схлопывание пропущенных индексов (канонизация)",
			input: "items=third&items=first&items=second",
			want:  "items=first&items=second&items=third",
		},
		{
			name:  "Многомерные структуры с текстом внутри скобок (не массивы)",
			input: "users[info][name]=bob&users[info][age]=25",
			want:  "users[info][age]=25&users[info][name]=bob",
		},
		{
			name:  "Декодирование URL-символов (%5B и %5D)",
			input: "sort=price_asc&filters%5Bkey%5D%5B1%5D=termal&filters%5Bkey%5D%5B0%5D=awasp",
			want:  "filters[key]=awasp&filters[key]=termal&sort=price_asc",
		},
		{
			name:  "Пустая строка",
			input: "",
			want:  "",
		},
		{
			name:    "Ошибка при невалидном URL-экранировании",
			input:   "sort=price%asc",
			want:    "",
			wantErr: true,
		},
		// --- НОВЫЕ РАЗНООБРАЗНЫЕ ТЕСТЫ ---
		{
			name:  "Сложные спецсимволы в значениях (Unicode, эмодзи, пробелы)",
			input: "text=привет+мир&emoji=🚀&search=go+lang&symbols=a+b%26c",
			want:  "emoji=🚀&search=go lang&symbols=a b&c&text=привет мир",
		},
		{
			name:  "Полные дубликаты пар ключ-значение",
			input: "tag=go&tag=go&category=dev&tag=go",
			want:  "category=dev&tag=go&tag=go&tag=go",
		},
		{
			name:  "Индексы вперемешку с пустыми квадратными скобками",
			input: "tags[]=go&tags[0]=rust&tags[]=js",
			want:  "tags[0]=rust&tags[]=go&tags[]=js", // [] не число, остаются как есть и сортируются по значениям
		},
		{
			name:  "Многомерный массив, где индекс стоит в середине пути",
			input: "matrix[0][line]=2&matrix[1][line]=1",
			want:  "matrix[line]=1&matrix[line]=2", // Числовые индексы вырезаются, пути схлопываются, сортировка по значению
		},
		{
			name:  "Неполные или сломанные квадратные скобки",
			input: "invalid[key=1&broken]=2&correct[0]=3&unopened]=4",
			want:  "broken]=2&correct=3&invalid[key=1&unopened]=4",
		},
		{
			name:  "Отрицательные числа или текст с цифрами внутри скобок",
			input: "v[-1]=neg&v[123a]=text&v[0]=pos",
			want:  "v[-1]=neg&v[123a]=text&v=pos", // [-1] и [123a] не являются валидными индексами массива, [0] - вырезается
		},
		{
			name:  "Ключи, состоящие только из спецсимволов и амперсанда внутри значений",
			input: "%3F%23%3D=value&%26=amp&a=1",
			want:  "&=amp&?=value&a=1",
		},
		{
			name:  "Проверка полного пути в url",
			input: "/some/path?filters[key]=termal&filters[key]=awasp&filter[a]=2&filter[a]=1",
			want:  "/some/path?filter[a]=1&filter[a]=2&filters[key]=awasp&filters[key]=termal",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotEncoded, err := NormalizeQuery(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("NormalizeQuery() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr {
				return
			}

			// Декодируем для удобного сравнения строк в тесте
			got, err := url.QueryUnescape(gotEncoded)
			if err != nil {
				t.Fatalf("Не удалось декодировать результат: %v", err)
			}

			if got != tt.want {
				t.Errorf("\nПровал в: %s\nВход:      %s\nПолучено:  %s\nОжидалось: %s", tt.name, tt.input, got, tt.want)
			}
		})
	}
}

func TestFastNormalizeQuery_StrictOzonOrder(t *testing.T) {
	strictExpectedOrder := "currency_price=2500.000;363536.000&delivery=2&seria=76068002&writer=76067987"

	tests := []struct {
		name  string
		input string
	}{
		{
			name:  "Вариация 1: Оригинальный порядок Ozon (currency_price первый)",
			input: "currency_price=2500.000%3B363536.000&delivery=2&seria=76068002&writer=76067987",
		},
		{
			name:  "Вариация 2: Полностью перевернутый порядок (writer первый)",
			input: "writer=76067987&seria=76068002&delivery=2&currency_price=2500.000%3B363536.000",
		},
		{
			name:  "Вариация 3: Хаотичное перемешивание параметров",
			input: "delivery=2&writer=76067987&currency_price=2500.000%3B363536.000&seria=76068002",
		},
		{
			name:  "Вариация 4: Запрос с уже декодированными символами точки с запятой",
			input: "seria=76068002&delivery=2&writer=76067987&currency_price=2500.000;363536.000",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotEncoded, _ := NormalizeQuery(tt.input)
			gotDecoded, _ := url.QueryUnescape(gotEncoded)

			if gotDecoded != strictExpectedOrder {
				t.Errorf(
					"\n[НАРУШЕН ПОРЯДОК СОРТИРОВКИ]\nТест:      %s\nНа входе:  %s\nПолучено:  %s\nОжидалось: %s",
					tt.name, tt.input, gotDecoded, strictExpectedOrder,
				)
			}
		})
	}
}

func BenchmarkFastNormalizeQuery_Standard(b *testing.B) {
	input := "sort=price_asc&filters[key]=termal&filters[key]=awasp&position[key]"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = NormalizeQuery(input)
	}
}

func BenchmarkFastNormalizeQuery_Heavy(b *testing.B) {
	input := "sort=price_asc&filters[key]=termal&filters[key]=awasp&position[key]=top&page=2&limit=10&user[id]=123&items=third&items=first"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = NormalizeQuery(input)
	}
}

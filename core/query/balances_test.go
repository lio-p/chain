package query

import (
	"reflect"
	"testing"
	"time"

	"github.com/davecgh/go-spew/spew"
	"golang.org/x/net/context"

	"chain/core/account"
	"chain/core/asset"
	"chain/core/asset/assettest"
	"chain/core/blocksigner"
	"chain/core/generator"
	"chain/core/query/chql"
	"chain/core/txdb"
	"chain/cos"
	"chain/cos/bc"
	"chain/crypto/ed25519"
	"chain/database/pg"
	"chain/database/pg/pgtest"
	"chain/testutil"
)

func TestConstructBalancesQuery(t *testing.T) {
	now := uint64(123456)
	testCases := []struct {
		query      string
		values     []interface{}
		wantQuery  string
		wantValues []interface{}
	}{
		{
			query:      "asset_id = $1 AND account_id = 'abc'",
			wantQuery:  `SELECT COALESCE(SUM((data->>'amount')::integer), 0), "data"->>'asset_id' FROM "annotated_outputs" WHERE ((data @> $1::jsonb)) AND timespan @> $2::int8 GROUP BY 2`,
			wantValues: []interface{}{`{"account_id":"abc"}`, now},
		},
		{
			query:      "asset_id = $1 AND account_id = $2",
			values:     []interface{}{"foo", "bar"},
			wantQuery:  `SELECT COALESCE(SUM((data->>'amount')::integer), 0) FROM "annotated_outputs" WHERE ((data @> $1::jsonb)) AND timespan @> $2::int8`,
			wantValues: []interface{}{`{"account_id":"bar","asset_id":"foo"}`, now},
		},
	}

	for i, tc := range testCases {
		q, err := chql.Parse(tc.query)
		if err != nil {
			t.Fatal(err)
		}
		expr, err := chql.AsSQL(q, "data", tc.values)
		if err != nil {
			t.Fatal(err)
		}
		query, values := constructBalancesQuery(expr, now)
		if query != tc.wantQuery {
			t.Errorf("case %d: got\n%s\nwant\n%s", i, query, tc.wantQuery)
		}
		if !reflect.DeepEqual(values, tc.wantValues) {
			t.Errorf("case %d: got %#v, want %#v", i, values, tc.wantValues)
		}
	}
}

// TODO(bobg): This is largely copy-pasta from
// TestQueryOutputs. Factor out the common bits.
func TestQueryBalances(t *testing.T) {
	type (
		assetAccountAmount struct {
			bc.AssetAmount
			AccountID string
		}
		testcase struct {
			query  string
			values []interface{}
			when   time.Time
			want   []interface{}
		}
	)

	time1 := time.Now()

	_, db := pgtest.NewDB(t, pgtest.SchemaPath)
	ctx := pg.NewContext(context.Background(), db)
	store, pool := txdb.New(db)
	fc, err := cos.NewFC(ctx, store, pool, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	asset.Init(fc, true)
	account.Init(fc)
	localSigner := blocksigner.New(testutil.TestPrv, db, fc)
	g := &generator.Generator{
		Config: generator.Config{
			LocalSigner:  localSigner,
			BlockPeriod:  time.Second,
			BlockKeys:    []ed25519.PublicKey{testutil.TestPub},
			SigsRequired: 1,
			FC:           fc,
		},
	}
	genesis, err := fc.UpsertGenesisBlock(ctx, []ed25519.PublicKey{testutil.TestPub}, 1, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	genesisHash := genesis.Hash()

	indexer := NewIndexer(db, fc)

	acct1, err := account.Create(ctx, []string{testutil.TestXPub.String()}, 1, nil, nil)
	if err != nil {
		t.Fatal(err)
	}

	acct2, err := account.Create(ctx, []string{testutil.TestXPub.String()}, 1, nil, nil)
	if err != nil {
		t.Fatal(err)
	}

	asset1, err := asset.Define(ctx, []string{testutil.TestXPub.String()}, 1, nil, genesisHash, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	asset2, err := asset.Define(ctx, []string{testutil.TestXPub.String()}, 1, nil, genesisHash, nil, nil)
	if err != nil {
		t.Fatal(err)
	}

	assettest.IssueAssetsFixture(ctx, t, fc, asset1.AssetID, 867, acct1.ID)

	_, err = g.MakeBlock(ctx)
	if err != nil {
		t.Fatal(err)
	}

	time2 := time.Now()

	want0 := interface{}(map[string]interface{}{"amount": uint64(0)})
	want867 := interface{}(map[string]interface{}{"amount": uint64(867)})

	cases := []testcase{
		{
			query:  "asset_id = $1",
			values: []interface{}{asset1.AssetID.String()},
			when:   time1,
			want:   []interface{}{want0},
		},
		{
			query:  "asset_id = $1",
			values: []interface{}{asset1.AssetID.String()},
			when:   time2,
			want:   []interface{}{want867},
		},
		{
			query:  "asset_id = $1",
			values: []interface{}{asset2.AssetID.String()},
			when:   time1,
			want:   []interface{}{want0},
		},
		{
			query:  "asset_id = $1",
			values: []interface{}{asset2.AssetID.String()},
			when:   time2,
			want:   []interface{}{want0},
		},
		{
			query:  "account_id = $1",
			values: []interface{}{acct1.ID},
			when:   time1,
			want:   []interface{}{want0},
		},
		{
			query:  "account_id = $1",
			values: []interface{}{acct1.ID},
			when:   time2,
			want:   []interface{}{want867},
		},
		{
			query:  "account_id = $1",
			values: []interface{}{acct2.ID},
			when:   time1,
			want:   []interface{}{want0},
		},
		{
			query:  "account_id = $1",
			values: []interface{}{acct2.ID},
			when:   time2,
			want:   []interface{}{want0},
		},
		{
			query:  "asset_id = $1 AND account_id = $2",
			values: []interface{}{asset1.AssetID.String(), acct1.ID},
			when:   time2,
			want:   []interface{}{want867},
		},
		{
			query:  "asset_id = $1 AND account_id = $2",
			values: []interface{}{asset2.AssetID.String(), acct1.ID},
			when:   time2,
			want:   []interface{}{want0},
		},
	}

	for i, tc := range cases {
		chql, err := chql.Parse(tc.query)
		if err != nil {
			t.Fatal(err)
		}
		balances, err := indexer.Balances(ctx, chql, tc.values, bc.Millis(tc.when))
		if err != nil {
			t.Fatal(err)
		}
		if len(balances) != len(tc.want) {
			t.Fatalf("case %d: got %d balances, want %d", i, len(balances), len(tc.want))
		}
		if !reflect.DeepEqual(balances, tc.want) {
			t.Errorf("case %d: got:\n%s\nwant:\n%s", i, spew.Sdump(balances), spew.Sdump(tc.want))
		}
	}
}
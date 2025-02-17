#![no_std]
use soroban_sdk::{
    contract, contractimpl, contracttype, token, Address, Env,
};

#[contracttype]
pub enum DataKey {
    Token,
}

fn get_token(e: &Env) -> Address {
    e.storage().persistent().get(&DataKey::Token).unwrap()
}

#[contract]
pub struct SACTest;

#[contractimpl]
impl SACTest {

    pub fn init(e: Env, contract: Address) {
        e.storage().persistent().set(&DataKey::Token, &contract);
    }

    pub fn get_token(e: Env) -> Address {
        get_token(&e)
    }

    pub fn burn_self(env: Env, amount: i128) {
        let client = token::Client::new(&env, &get_token(&env));
        client.burn(&env.current_contract_address(), &amount);
    }

    pub fn transfer(env: Env, to: Address, amount: i128) {
        let client = token::Client::new(&env, &get_token(&env));
        client.transfer(&env.current_contract_address(), &to, &amount);
    }
}

#[test]
fn test() {
    use soroban_sdk::testutils::Address as _;

    let env = Env::default();
    env.mock_all_auths();
    let admin = Address::random(&env);
    let token_contract_id = env.register_stellar_asset_contract(admin.clone());

    let contract_address = env.register_contract(None, SACTest);
    let contract = SACTestClient::new(&env, &contract_address);
    
    contract.init(&token_contract_id);

    let token = token::Client::new(&env, &contract.get_token());
    let token_admin = token::AdminClient::new(&env, &contract.get_token());
    assert_eq!(token.decimals(), 7);
    
    token_admin.mint(&contract_address, &1000);

    contract.burn_self(&400);
    assert_eq!(token.balance(&contract_address), 600);

    let user = Address::random(&env);
    contract.transfer(&user, &100);
    assert_eq!(token.balance(&contract_address), 500);
    assert_eq!(token.balance(&user), 100);
}

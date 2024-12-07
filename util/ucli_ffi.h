#ifndef UCLI_FFI_H
#define UCLI_FFI_H
struct UltraMarketPublisher {
    void* topic;
    void* inner;
};

struct UltraOrderPublisher {
    void* topic;
    void* inner;
};

struct UltraMarketReceiver {
    void* topic;
    void* inner;
};

struct UltraOrderReceiver {
    void* topic;
    void* inner;
};



void init_market_publisher(char* topic, struct UltraMarketPublisher* publisher);
void init_order_publisher(char* topic, struct UltraOrderPublisher* publisher);
void init_market_receiver(char* topic, struct UltraMarketReceiver* receiver);
void init_order_receiver(char* topic, struct UltraOrderReceiver* receiver);

void publish_market(struct UltraMarketPublisher* publisher, char* msg);
void publish_order(struct UltraOrderPublisher* publisher, char* msg);
void receive_market(struct UltraMarketReceiver* receiver, char* buf);
void receive_order(struct UltraOrderReceiver* receiver, char* buf);

#endif //UCLI_FFI_H

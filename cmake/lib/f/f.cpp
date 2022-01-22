#include <iostream>
#include <nlohmann/json.hpp>
#include "lib/f/f.h"

using namespace nlohmann;

void putout(std::ostream &out) {
	out << json{{"val", "hello"}};
}
